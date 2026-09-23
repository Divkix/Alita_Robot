------------------------------ MODULE FloodReset ------------------------------
(* Model of updateFloodWithSettings in alita/modules/antiflood.go.
   One user's flood slot is a pointer in a sync.Map. The burst window is Idle
   seconds. The cleaner only deletes the pointer after a much longer delay, so
   an idle slot is still present.

   The implementation's commit is:
     absent  -> LoadOrStore (inserts only when the key is missing)
     present -> CompareAndSwap of the loaded pointer

   Buggy = TRUE is the code that treats an idle-but-present slot as absent and
   then calls LoadOrStore. That call cannot overwrite the pointer, so the
   retry loop never commits.

   Buggy = FALSE CAS-replaces the idle pointer with a fresh count of 1.
 *)

EXTENDS Integers, TLC

CONSTANT Limit, Idle, CleanAfter, MaxTries, Buggy

ASSUME /\ Limit \in Nat /\ Limit >= 1
       /\ Idle \in Nat /\ Idle >= 1
       /\ CleanAfter \in Nat /\ CleanAfter > Idle
       /\ MaxTries \in Nat /\ MaxTries >= 1
       /\ Buggy \in BOOLEAN

VARIABLES present, gen, count, age, tries, done, observed

vars == <<present, gen, count, age, tries, done, observed>>

TypeOK ==
    /\ present \in BOOLEAN
    /\ gen \in Nat
    /\ count \in 0..(Limit + 1)
    /\ age \in 0..CleanAfter
    /\ tries \in 0..(MaxTries + 1)
    /\ done \in BOOLEAN
    /\ observed \in 0..(Limit + 1)

(* The counterexample starts from the state the Go map actually holds:
   a pointer written on the previous burst, still present, idle past the
   60s window, and not yet old enough for cleanup. *)
Init ==
    /\ present = TRUE
    /\ gen = 0
    /\ count = 1
    /\ age = Idle + 1
    /\ tries = 0
    /\ done = FALSE
    /\ observed = 0

IdleSlot == present /\ age > Idle

Store(nextCount, nextAge, seen) ==
    /\ present' = TRUE
    /\ gen' = gen + 1
    /\ count' = nextCount
    /\ age' = nextAge
    /\ done' = TRUE
    /\ observed' = seen

(* One iteration of the retry loop. A failed commit leaves the slot untouched
   and increments tries, which is how the Go loop spins. *)
Update ==
    /\ done = FALSE
    /\ tries' = tries + 1
    /\ IF ~present
       THEN Store(1, 0, 1)
       ELSE IF IdleSlot
            THEN IF Buggy
                 THEN UNCHANGED <<present, gen, count, age, done, observed>>
                 ELSE Store(1, 0, 1)
            ELSE LET n == count + 1
                 IN IF n > Limit
                    THEN Store(0, 0, n)
                    ELSE Store(n, 0, n)

(* cleanupOnce deletes only long-idle pointers. It does not run for the
   60s burst reset, which is why it cannot unblock the buggy updater. *)
Clean ==
    /\ present
    /\ age > CleanAfter
    /\ present' = FALSE
    /\ gen' = gen + 1
    /\ count' = 0
    /\ age' = 0
    /\ UNCHANGED <<tries, done, observed>>

Next == Update \/ Clean \/ (done /\ UNCHANGED vars)

Spec == Init /\ [][Next]_vars

(* The handler must commit within a handful of CAS attempts. *)
ProgressInv == tries <= MaxTries

(* A commit from the idle initial state stores a fresh one-message window. *)
IdleResetInv == done => observed = 1 /\ count = 1 /\ age = 0 /\ present

=============================================================================
