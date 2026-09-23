/-
  The commit step of `updateFloodWithSettings`.

  `idleFor` is `now - lastActivity` for a pointer that is still in the map.
  The burst window treats `idleFor > 60` as a new window. The map cleaner does
  not remove that pointer until a later deadline, so the key is still present.

  `buggyStep` is the old commit: an idle key is treated as absent, and
  `LoadOrStore` returns the existing pointer instead of writing. The result is
  `none` — the retry loop does not commit.

  `fixedStep` CAS-replaces that same pointer with a fresh count of 1.
-/

structure Slot where
  present : Bool
  count : Nat
  age : Nat
deriving DecidableEq, Repr

structure Outcome where
  slot : Slot
  observed : Nat
  punish : Bool
deriving DecidableEq, Repr

def idleThreshold : Nat := 60

def buggyStep (s : Slot) (limit idleFor : Nat) : Option Outcome :=
  if s.present && idleThreshold < idleFor then
    none
  else if !s.present then
    some ⟨⟨true, 1, 0⟩, 1, false⟩
  else
    let n := s.count + 1
    if limit < n then
      some ⟨⟨true, 0, 0⟩, n, true⟩
    else
      some ⟨⟨true, n, 0⟩, n, false⟩

def fixedStep (s : Slot) (limit idleFor : Nat) : Outcome :=
  if s.present = false then
    ⟨⟨true, 1, 0⟩, 1, false⟩
  else if idleThreshold < idleFor then
    ⟨⟨true, 1, 0⟩, 1, false⟩
  else if limit < s.count + 1 then
    ⟨⟨true, 0, 0⟩, s.count + 1, true⟩
  else
    ⟨⟨true, s.count + 1, 0⟩, s.count + 1, false⟩

/-- An idle but present counter makes the buggy commit fail for every count. -/
theorem buggy_idle_does_not_commit (count limit : Nat) :
    buggyStep ⟨true, count, 61⟩ limit 61 = none := by
  simp [buggyStep, idleThreshold]

/-- The fixed commit replaces that pointer with a one-message window. -/
theorem fixed_idle_resets (count limit : Nat) :
    fixedStep ⟨true, count, 61⟩ limit 61 =
      ⟨⟨true, 1, 0⟩, 1, false⟩ := by
  simp [fixedStep, idleThreshold]

/-- A live counter above the limit is punished and the stored count is cleared. -/
theorem fixed_over_limit_punishes :
    fixedStep ⟨true, 5, 0⟩ 5 0 = ⟨⟨true, 0, 0⟩, 6, true⟩ := by
  native_decide

/-- After a commit, the stored count never exceeds the configured limit. -/
theorem fixed_stored_count_le_limit (s : Slot) (limit idleFor : Nat)
    (hLimit : 1 ≤ limit) :
    (fixedStep s limit idleFor).slot.count ≤ limit := by
  unfold fixedStep
  split
  · simp
    omega
  · split
    · simp
      omega
    · split
      · simp
      · simp
        omega

#guard buggyStep ⟨true, 4, 61⟩ 5 61 == none
#guard fixedStep ⟨true, 4, 61⟩ 5 61 == ⟨⟨true, 1, 0⟩, 1, false⟩
#guard fixedStep ⟨true, 5, 10⟩ 5 10 == ⟨⟨true, 0, 0⟩, 6, true⟩
#guard fixedStep ⟨false, 0, 0⟩ 5 0 == ⟨⟨true, 1, 0⟩, 1, false⟩
