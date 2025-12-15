create sequence if not exists "public"."admin_id_seq";

create sequence if not exists "public"."antiflood_settings_id_seq";

create sequence if not exists "public"."blacklists_id_seq";

create sequence if not exists "public"."channels_id_seq";

create sequence if not exists "public"."chats_id_seq";

create sequence if not exists "public"."connection_id_seq";

create sequence if not exists "public"."connection_settings_id_seq";

create sequence if not exists "public"."devs_id_seq";

create sequence if not exists "public"."disable_id_seq";

create sequence if not exists "public"."filters_id_seq";

create sequence if not exists "public"."greetings_id_seq";

create sequence if not exists "public"."locks_id_seq";

create sequence if not exists "public"."notes_id_seq";

create sequence if not exists "public"."notes_settings_id_seq";

create sequence if not exists "public"."pins_id_seq";

create sequence if not exists "public"."report_chat_settings_id_seq";

create sequence if not exists "public"."report_user_settings_id_seq";

create sequence if not exists "public"."rules_id_seq";

create sequence if not exists "public"."users_id_seq";

create sequence if not exists "public"."warns_settings_id_seq";

create sequence if not exists "public"."warns_users_id_seq";

create table "public"."admin" (
    "id" bigint not null default nextval('admin_id_seq'::regclass),
    "chat_id" bigint not null,
    "anon_admin" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."antiflood_settings" (
    "id" bigint not null default nextval('antiflood_settings_id_seq'::regclass),
    "chat_id" bigint not null,
    "limit" bigint default 5,
    "action" text default 'mute'::text,
    "mode" text default 'mute'::text,
    "delete_antiflood_message" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone,
    "flood_limit" bigint default 5
);


create table "public"."blacklists" (
    "id" bigint not null default nextval('blacklists_id_seq'::regclass),
    "chat_id" bigint not null,
    "word" text not null,
    "action" text default 'warn'::text,
    "reason" text,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."channels" (
    "id" bigint not null default nextval('channels_id_seq'::regclass),
    "chat_id" bigint not null,
    "channel_id" bigint,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."chat_users" (
    "chat_id" bigint not null,
    "user_id" bigint not null
);


create table "public"."chats" (
    "id" bigint not null default nextval('chats_id_seq'::regclass),
    "chat_id" bigint not null,
    "chat_name" text,
    "language" text,
    "users" jsonb,
    "is_inactive" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."connection" (
    "id" bigint not null default nextval('connection_id_seq'::regclass),
    "user_id" bigint not null,
    "chat_id" bigint not null,
    "connected" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."connection_settings" (
    "id" bigint not null default nextval('connection_settings_id_seq'::regclass),
    "chat_id" bigint not null,
    "enabled" boolean default true,
    "allow_connect" boolean default true,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."devs" (
    "id" bigint not null default nextval('devs_id_seq'::regclass),
    "user_id" bigint not null,
    "is_dev" boolean default false,
    "dev" boolean default false,
    "sudo" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."disable" (
    "id" bigint not null default nextval('disable_id_seq'::regclass),
    "chat_id" bigint not null,
    "command" text not null,
    "disabled" boolean default true,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."filters" (
    "id" bigint not null default nextval('filters_id_seq'::regclass),
    "chat_id" bigint not null,
    "keyword" text not null,
    "filter_reply" text,
    "msgtype" bigint,
    "fileid" text,
    "nonotif" boolean default false,
    "filter_buttons" jsonb,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."greetings" (
    "id" bigint not null default nextval('greetings_id_seq'::regclass),
    "chat_id" bigint not null,
    "clean_service_settings" boolean default false,
    "welcome_clean_old" boolean default false,
    "welcome_last_msg_id" bigint,
    "welcome_enabled" boolean default true,
    "welcome_text" text,
    "welcome_file_id" text,
    "welcome_type" bigint,
    "welcome_btns" jsonb,
    "goodbye_clean_old" boolean default false,
    "goodbye_last_msg_id" bigint,
    "goodbye_enabled" boolean default true,
    "goodbye_text" text,
    "goodbye_file_id" text,
    "goodbye_type" bigint,
    "goodbye_btns" jsonb,
    "auto_approve" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."locks" (
    "id" bigint not null default nextval('locks_id_seq'::regclass),
    "chat_id" bigint not null,
    "lock_type" text not null,
    "locked" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."notes" (
    "id" bigint not null default nextval('notes_id_seq'::regclass),
    "chat_id" bigint not null,
    "note_name" text not null,
    "note_content" text,
    "file_id" text,
    "msg_type" bigint,
    "buttons" jsonb,
    "admin_only" boolean default false,
    "private_only" boolean default false,
    "group_only" boolean default false,
    "web_preview" boolean default true,
    "is_protected" boolean default false,
    "no_notif" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."notes_settings" (
    "id" bigint not null default nextval('notes_settings_id_seq'::regclass),
    "chat_id" bigint not null,
    "private" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."pins" (
    "id" bigint not null default nextval('pins_id_seq'::regclass),
    "chat_id" bigint not null,
    "msg_id" bigint,
    "clean_linked" boolean default false,
    "anti_channel_pin" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."report_chat_settings" (
    "id" bigint not null default nextval('report_chat_settings_id_seq'::regclass),
    "chat_id" bigint not null,
    "enabled" boolean default true,
    "status" boolean default true,
    "blocked_list" jsonb,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."report_user_settings" (
    "id" bigint not null default nextval('report_user_settings_id_seq'::regclass),
    "user_id" bigint not null,
    "enabled" boolean default true,
    "status" boolean default true,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."rules" (
    "id" bigint not null default nextval('rules_id_seq'::regclass),
    "chat_id" bigint not null,
    "rules" text,
    "rules_btn" text,
    "private" boolean default false,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."users" (
    "id" bigint not null default nextval('users_id_seq'::regclass),
    "user_id" bigint not null,
    "username" text,
    "name" text,
    "language" text default 'en'::text,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."warns_settings" (
    "id" bigint not null default nextval('warns_settings_id_seq'::regclass),
    "chat_id" bigint not null,
    "warn_limit" bigint default 3,
    "warn_mode" text,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


create table "public"."warns_users" (
    "id" bigint not null default nextval('warns_users_id_seq'::regclass),
    "user_id" bigint not null,
    "chat_id" bigint not null,
    "num_warns" bigint default 0,
    "warns" jsonb,
    "created_at" timestamp with time zone,
    "updated_at" timestamp with time zone
);


alter sequence "public"."admin_id_seq" owned by "public"."admin"."id";

alter sequence "public"."antiflood_settings_id_seq" owned by "public"."antiflood_settings"."id";

alter sequence "public"."blacklists_id_seq" owned by "public"."blacklists"."id";

alter sequence "public"."channels_id_seq" owned by "public"."channels"."id";

alter sequence "public"."chats_id_seq" owned by "public"."chats"."id";

alter sequence "public"."connection_id_seq" owned by "public"."connection"."id";

alter sequence "public"."connection_settings_id_seq" owned by "public"."connection_settings"."id";

alter sequence "public"."devs_id_seq" owned by "public"."devs"."id";

alter sequence "public"."disable_id_seq" owned by "public"."disable"."id";

alter sequence "public"."filters_id_seq" owned by "public"."filters"."id";

alter sequence "public"."greetings_id_seq" owned by "public"."greetings"."id";

alter sequence "public"."locks_id_seq" owned by "public"."locks"."id";

alter sequence "public"."notes_id_seq" owned by "public"."notes"."id";

alter sequence "public"."notes_settings_id_seq" owned by "public"."notes_settings"."id";

alter sequence "public"."pins_id_seq" owned by "public"."pins"."id";

alter sequence "public"."report_chat_settings_id_seq" owned by "public"."report_chat_settings"."id";

alter sequence "public"."report_user_settings_id_seq" owned by "public"."report_user_settings"."id";

alter sequence "public"."rules_id_seq" owned by "public"."rules"."id";

alter sequence "public"."users_id_seq" owned by "public"."users"."id";

alter sequence "public"."warns_settings_id_seq" owned by "public"."warns_settings"."id";

alter sequence "public"."warns_users_id_seq" owned by "public"."warns_users"."id";

CREATE UNIQUE INDEX admin_pkey ON public.admin USING btree (id);

CREATE UNIQUE INDEX antiflood_settings_pkey ON public.antiflood_settings USING btree (id);

CREATE UNIQUE INDEX blacklists_pkey ON public.blacklists USING btree (id);

CREATE UNIQUE INDEX channels_pkey ON public.channels USING btree (id);

CREATE UNIQUE INDEX chat_users_pkey ON public.chat_users USING btree (chat_id, user_id);

CREATE UNIQUE INDEX chats_pkey ON public.chats USING btree (id);

CREATE UNIQUE INDEX connection_pkey ON public.connection USING btree (id);

CREATE UNIQUE INDEX connection_settings_pkey ON public.connection_settings USING btree (id);

CREATE UNIQUE INDEX devs_pkey ON public.devs USING btree (id);

CREATE UNIQUE INDEX disable_pkey ON public.disable USING btree (id);

CREATE UNIQUE INDEX filters_pkey ON public.filters USING btree (id);

CREATE UNIQUE INDEX greetings_pkey ON public.greetings USING btree (id);

CREATE UNIQUE INDEX idx_admin_chat_id ON public.admin USING btree (chat_id);

CREATE UNIQUE INDEX idx_antiflood_settings_chat_id ON public.antiflood_settings USING btree (chat_id);

CREATE INDEX idx_blacklist_chat_word ON public.blacklists USING btree (chat_id, word);

CREATE UNIQUE INDEX idx_channels_chat_id ON public.channels USING btree (chat_id);

CREATE UNIQUE INDEX idx_chats_chat_id ON public.chats USING btree (chat_id);

CREATE UNIQUE INDEX idx_connection_settings_chat_id ON public.connection_settings USING btree (chat_id);

CREATE INDEX idx_connection_user_chat ON public.connection USING btree (user_id, chat_id);

CREATE UNIQUE INDEX idx_devs_user_id ON public.devs USING btree (user_id);

CREATE INDEX idx_disable_chat_command ON public.disable USING btree (chat_id, command);

CREATE INDEX idx_filters_chat_keyword ON public.filters USING btree (chat_id, keyword);

CREATE UNIQUE INDEX idx_greetings_chat_id ON public.greetings USING btree (chat_id);

CREATE INDEX idx_lock_chat_type ON public.locks USING btree (chat_id, lock_type);

CREATE INDEX idx_notes_chat_name ON public.notes USING btree (chat_id, note_name);

CREATE UNIQUE INDEX idx_notes_settings_chat_id ON public.notes_settings USING btree (chat_id);

CREATE UNIQUE INDEX idx_pins_chat_id ON public.pins USING btree (chat_id);

CREATE UNIQUE INDEX idx_report_chat_settings_chat_id ON public.report_chat_settings USING btree (chat_id);

CREATE UNIQUE INDEX idx_report_user_settings_user_id ON public.report_user_settings USING btree (user_id);

CREATE UNIQUE INDEX idx_rules_chat_id ON public.rules USING btree (chat_id);

CREATE UNIQUE INDEX idx_users_user_id ON public.users USING btree (user_id);

CREATE INDEX idx_users_user_name ON public.users USING btree (username);

CREATE UNIQUE INDEX idx_warns_settings_chat_id ON public.warns_settings USING btree (chat_id);

CREATE INDEX idx_warns_user_chat ON public.warns_users USING btree (user_id, chat_id);

CREATE UNIQUE INDEX locks_pkey ON public.locks USING btree (id);

CREATE UNIQUE INDEX notes_pkey ON public.notes USING btree (id);

CREATE UNIQUE INDEX notes_settings_pkey ON public.notes_settings USING btree (id);

CREATE UNIQUE INDEX pins_pkey ON public.pins USING btree (id);

CREATE UNIQUE INDEX report_chat_settings_pkey ON public.report_chat_settings USING btree (id);

CREATE UNIQUE INDEX report_user_settings_pkey ON public.report_user_settings USING btree (id);

CREATE UNIQUE INDEX rules_pkey ON public.rules USING btree (id);

CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id);

CREATE UNIQUE INDEX warns_settings_pkey ON public.warns_settings USING btree (id);

CREATE UNIQUE INDEX warns_users_pkey ON public.warns_users USING btree (id);

alter table "public"."admin" add constraint "admin_pkey" PRIMARY KEY using index "admin_pkey";

alter table "public"."antiflood_settings" add constraint "antiflood_settings_pkey" PRIMARY KEY using index "antiflood_settings_pkey";

alter table "public"."blacklists" add constraint "blacklists_pkey" PRIMARY KEY using index "blacklists_pkey";

alter table "public"."channels" add constraint "channels_pkey" PRIMARY KEY using index "channels_pkey";

alter table "public"."chat_users" add constraint "chat_users_pkey" PRIMARY KEY using index "chat_users_pkey";

alter table "public"."chats" add constraint "chats_pkey" PRIMARY KEY using index "chats_pkey";

alter table "public"."connection" add constraint "connection_pkey" PRIMARY KEY using index "connection_pkey";

alter table "public"."connection_settings" add constraint "connection_settings_pkey" PRIMARY KEY using index "connection_settings_pkey";

alter table "public"."devs" add constraint "devs_pkey" PRIMARY KEY using index "devs_pkey";

alter table "public"."disable" add constraint "disable_pkey" PRIMARY KEY using index "disable_pkey";

alter table "public"."filters" add constraint "filters_pkey" PRIMARY KEY using index "filters_pkey";

alter table "public"."greetings" add constraint "greetings_pkey" PRIMARY KEY using index "greetings_pkey";

alter table "public"."locks" add constraint "locks_pkey" PRIMARY KEY using index "locks_pkey";

alter table "public"."notes" add constraint "notes_pkey" PRIMARY KEY using index "notes_pkey";

alter table "public"."notes_settings" add constraint "notes_settings_pkey" PRIMARY KEY using index "notes_settings_pkey";

alter table "public"."pins" add constraint "pins_pkey" PRIMARY KEY using index "pins_pkey";

alter table "public"."report_chat_settings" add constraint "report_chat_settings_pkey" PRIMARY KEY using index "report_chat_settings_pkey";

alter table "public"."report_user_settings" add constraint "report_user_settings_pkey" PRIMARY KEY using index "report_user_settings_pkey";

alter table "public"."rules" add constraint "rules_pkey" PRIMARY KEY using index "rules_pkey";

alter table "public"."users" add constraint "users_pkey" PRIMARY KEY using index "users_pkey";

alter table "public"."warns_settings" add constraint "warns_settings_pkey" PRIMARY KEY using index "warns_settings_pkey";

alter table "public"."warns_users" add constraint "warns_users_pkey" PRIMARY KEY using index "warns_users_pkey";

grant delete on table "public"."admin" to "anon";

grant insert on table "public"."admin" to "anon";

grant references on table "public"."admin" to "anon";

grant select on table "public"."admin" to "anon";

grant trigger on table "public"."admin" to "anon";

grant truncate on table "public"."admin" to "anon";

grant update on table "public"."admin" to "anon";

grant delete on table "public"."admin" to "authenticated";

grant insert on table "public"."admin" to "authenticated";

grant references on table "public"."admin" to "authenticated";

grant select on table "public"."admin" to "authenticated";

grant trigger on table "public"."admin" to "authenticated";

grant truncate on table "public"."admin" to "authenticated";

grant update on table "public"."admin" to "authenticated";

grant delete on table "public"."admin" to "service_role";

grant insert on table "public"."admin" to "service_role";

grant references on table "public"."admin" to "service_role";

grant select on table "public"."admin" to "service_role";

grant trigger on table "public"."admin" to "service_role";

grant truncate on table "public"."admin" to "service_role";

grant update on table "public"."admin" to "service_role";

grant delete on table "public"."antiflood_settings" to "anon";

grant insert on table "public"."antiflood_settings" to "anon";

grant references on table "public"."antiflood_settings" to "anon";

grant select on table "public"."antiflood_settings" to "anon";

grant trigger on table "public"."antiflood_settings" to "anon";

grant truncate on table "public"."antiflood_settings" to "anon";

grant update on table "public"."antiflood_settings" to "anon";

grant delete on table "public"."antiflood_settings" to "authenticated";

grant insert on table "public"."antiflood_settings" to "authenticated";

grant references on table "public"."antiflood_settings" to "authenticated";

grant select on table "public"."antiflood_settings" to "authenticated";

grant trigger on table "public"."antiflood_settings" to "authenticated";

grant truncate on table "public"."antiflood_settings" to "authenticated";

grant update on table "public"."antiflood_settings" to "authenticated";

grant delete on table "public"."antiflood_settings" to "service_role";

grant insert on table "public"."antiflood_settings" to "service_role";

grant references on table "public"."antiflood_settings" to "service_role";

grant select on table "public"."antiflood_settings" to "service_role";

grant trigger on table "public"."antiflood_settings" to "service_role";

grant truncate on table "public"."antiflood_settings" to "service_role";

grant update on table "public"."antiflood_settings" to "service_role";

grant delete on table "public"."blacklists" to "anon";

grant insert on table "public"."blacklists" to "anon";

grant references on table "public"."blacklists" to "anon";

grant select on table "public"."blacklists" to "anon";

grant trigger on table "public"."blacklists" to "anon";

grant truncate on table "public"."blacklists" to "anon";

grant update on table "public"."blacklists" to "anon";

grant delete on table "public"."blacklists" to "authenticated";

grant insert on table "public"."blacklists" to "authenticated";

grant references on table "public"."blacklists" to "authenticated";

grant select on table "public"."blacklists" to "authenticated";

grant trigger on table "public"."blacklists" to "authenticated";

grant truncate on table "public"."blacklists" to "authenticated";

grant update on table "public"."blacklists" to "authenticated";

grant delete on table "public"."blacklists" to "service_role";

grant insert on table "public"."blacklists" to "service_role";

grant references on table "public"."blacklists" to "service_role";

grant select on table "public"."blacklists" to "service_role";

grant trigger on table "public"."blacklists" to "service_role";

grant truncate on table "public"."blacklists" to "service_role";

grant update on table "public"."blacklists" to "service_role";

grant delete on table "public"."channels" to "anon";

grant insert on table "public"."channels" to "anon";

grant references on table "public"."channels" to "anon";

grant select on table "public"."channels" to "anon";

grant trigger on table "public"."channels" to "anon";

grant truncate on table "public"."channels" to "anon";

grant update on table "public"."channels" to "anon";

grant delete on table "public"."channels" to "authenticated";

grant insert on table "public"."channels" to "authenticated";

grant references on table "public"."channels" to "authenticated";

grant select on table "public"."channels" to "authenticated";

grant trigger on table "public"."channels" to "authenticated";

grant truncate on table "public"."channels" to "authenticated";

grant update on table "public"."channels" to "authenticated";

grant delete on table "public"."channels" to "service_role";

grant insert on table "public"."channels" to "service_role";

grant references on table "public"."channels" to "service_role";

grant select on table "public"."channels" to "service_role";

grant trigger on table "public"."channels" to "service_role";

grant truncate on table "public"."channels" to "service_role";

grant update on table "public"."channels" to "service_role";

grant delete on table "public"."chat_users" to "anon";

grant insert on table "public"."chat_users" to "anon";

grant references on table "public"."chat_users" to "anon";

grant select on table "public"."chat_users" to "anon";

grant trigger on table "public"."chat_users" to "anon";

grant truncate on table "public"."chat_users" to "anon";

grant update on table "public"."chat_users" to "anon";

grant delete on table "public"."chat_users" to "authenticated";

grant insert on table "public"."chat_users" to "authenticated";

grant references on table "public"."chat_users" to "authenticated";

grant select on table "public"."chat_users" to "authenticated";

grant trigger on table "public"."chat_users" to "authenticated";

grant truncate on table "public"."chat_users" to "authenticated";

grant update on table "public"."chat_users" to "authenticated";

grant delete on table "public"."chat_users" to "service_role";

grant insert on table "public"."chat_users" to "service_role";

grant references on table "public"."chat_users" to "service_role";

grant select on table "public"."chat_users" to "service_role";

grant trigger on table "public"."chat_users" to "service_role";

grant truncate on table "public"."chat_users" to "service_role";

grant update on table "public"."chat_users" to "service_role";

grant delete on table "public"."chats" to "anon";

grant insert on table "public"."chats" to "anon";

grant references on table "public"."chats" to "anon";

grant select on table "public"."chats" to "anon";

grant trigger on table "public"."chats" to "anon";

grant truncate on table "public"."chats" to "anon";

grant update on table "public"."chats" to "anon";

grant delete on table "public"."chats" to "authenticated";

grant insert on table "public"."chats" to "authenticated";

grant references on table "public"."chats" to "authenticated";

grant select on table "public"."chats" to "authenticated";

grant trigger on table "public"."chats" to "authenticated";

grant truncate on table "public"."chats" to "authenticated";

grant update on table "public"."chats" to "authenticated";

grant delete on table "public"."chats" to "service_role";

grant insert on table "public"."chats" to "service_role";

grant references on table "public"."chats" to "service_role";

grant select on table "public"."chats" to "service_role";

grant trigger on table "public"."chats" to "service_role";

grant truncate on table "public"."chats" to "service_role";

grant update on table "public"."chats" to "service_role";

grant delete on table "public"."connection" to "anon";

grant insert on table "public"."connection" to "anon";

grant references on table "public"."connection" to "anon";

grant select on table "public"."connection" to "anon";

grant trigger on table "public"."connection" to "anon";

grant truncate on table "public"."connection" to "anon";

grant update on table "public"."connection" to "anon";

grant delete on table "public"."connection" to "authenticated";

grant insert on table "public"."connection" to "authenticated";

grant references on table "public"."connection" to "authenticated";

grant select on table "public"."connection" to "authenticated";

grant trigger on table "public"."connection" to "authenticated";

grant truncate on table "public"."connection" to "authenticated";

grant update on table "public"."connection" to "authenticated";

grant delete on table "public"."connection" to "service_role";

grant insert on table "public"."connection" to "service_role";

grant references on table "public"."connection" to "service_role";

grant select on table "public"."connection" to "service_role";

grant trigger on table "public"."connection" to "service_role";

grant truncate on table "public"."connection" to "service_role";

grant update on table "public"."connection" to "service_role";

grant delete on table "public"."connection_settings" to "anon";

grant insert on table "public"."connection_settings" to "anon";

grant references on table "public"."connection_settings" to "anon";

grant select on table "public"."connection_settings" to "anon";

grant trigger on table "public"."connection_settings" to "anon";

grant truncate on table "public"."connection_settings" to "anon";

grant update on table "public"."connection_settings" to "anon";

grant delete on table "public"."connection_settings" to "authenticated";

grant insert on table "public"."connection_settings" to "authenticated";

grant references on table "public"."connection_settings" to "authenticated";

grant select on table "public"."connection_settings" to "authenticated";

grant trigger on table "public"."connection_settings" to "authenticated";

grant truncate on table "public"."connection_settings" to "authenticated";

grant update on table "public"."connection_settings" to "authenticated";

grant delete on table "public"."connection_settings" to "service_role";

grant insert on table "public"."connection_settings" to "service_role";

grant references on table "public"."connection_settings" to "service_role";

grant select on table "public"."connection_settings" to "service_role";

grant trigger on table "public"."connection_settings" to "service_role";

grant truncate on table "public"."connection_settings" to "service_role";

grant update on table "public"."connection_settings" to "service_role";

grant delete on table "public"."devs" to "anon";

grant insert on table "public"."devs" to "anon";

grant references on table "public"."devs" to "anon";

grant select on table "public"."devs" to "anon";

grant trigger on table "public"."devs" to "anon";

grant truncate on table "public"."devs" to "anon";

grant update on table "public"."devs" to "anon";

grant delete on table "public"."devs" to "authenticated";

grant insert on table "public"."devs" to "authenticated";

grant references on table "public"."devs" to "authenticated";

grant select on table "public"."devs" to "authenticated";

grant trigger on table "public"."devs" to "authenticated";

grant truncate on table "public"."devs" to "authenticated";

grant update on table "public"."devs" to "authenticated";

grant delete on table "public"."devs" to "service_role";

grant insert on table "public"."devs" to "service_role";

grant references on table "public"."devs" to "service_role";

grant select on table "public"."devs" to "service_role";

grant trigger on table "public"."devs" to "service_role";

grant truncate on table "public"."devs" to "service_role";

grant update on table "public"."devs" to "service_role";

grant delete on table "public"."disable" to "anon";

grant insert on table "public"."disable" to "anon";

grant references on table "public"."disable" to "anon";

grant select on table "public"."disable" to "anon";

grant trigger on table "public"."disable" to "anon";

grant truncate on table "public"."disable" to "anon";

grant update on table "public"."disable" to "anon";

grant delete on table "public"."disable" to "authenticated";

grant insert on table "public"."disable" to "authenticated";

grant references on table "public"."disable" to "authenticated";

grant select on table "public"."disable" to "authenticated";

grant trigger on table "public"."disable" to "authenticated";

grant truncate on table "public"."disable" to "authenticated";

grant update on table "public"."disable" to "authenticated";

grant delete on table "public"."disable" to "service_role";

grant insert on table "public"."disable" to "service_role";

grant references on table "public"."disable" to "service_role";

grant select on table "public"."disable" to "service_role";

grant trigger on table "public"."disable" to "service_role";

grant truncate on table "public"."disable" to "service_role";

grant update on table "public"."disable" to "service_role";

grant delete on table "public"."filters" to "anon";

grant insert on table "public"."filters" to "anon";

grant references on table "public"."filters" to "anon";

grant select on table "public"."filters" to "anon";

grant trigger on table "public"."filters" to "anon";

grant truncate on table "public"."filters" to "anon";

grant update on table "public"."filters" to "anon";

grant delete on table "public"."filters" to "authenticated";

grant insert on table "public"."filters" to "authenticated";

grant references on table "public"."filters" to "authenticated";

grant select on table "public"."filters" to "authenticated";

grant trigger on table "public"."filters" to "authenticated";

grant truncate on table "public"."filters" to "authenticated";

grant update on table "public"."filters" to "authenticated";

grant delete on table "public"."filters" to "service_role";

grant insert on table "public"."filters" to "service_role";

grant references on table "public"."filters" to "service_role";

grant select on table "public"."filters" to "service_role";

grant trigger on table "public"."filters" to "service_role";

grant truncate on table "public"."filters" to "service_role";

grant update on table "public"."filters" to "service_role";

grant delete on table "public"."greetings" to "anon";

grant insert on table "public"."greetings" to "anon";

grant references on table "public"."greetings" to "anon";

grant select on table "public"."greetings" to "anon";

grant trigger on table "public"."greetings" to "anon";

grant truncate on table "public"."greetings" to "anon";

grant update on table "public"."greetings" to "anon";

grant delete on table "public"."greetings" to "authenticated";

grant insert on table "public"."greetings" to "authenticated";

grant references on table "public"."greetings" to "authenticated";

grant select on table "public"."greetings" to "authenticated";

grant trigger on table "public"."greetings" to "authenticated";

grant truncate on table "public"."greetings" to "authenticated";

grant update on table "public"."greetings" to "authenticated";

grant delete on table "public"."greetings" to "service_role";

grant insert on table "public"."greetings" to "service_role";

grant references on table "public"."greetings" to "service_role";

grant select on table "public"."greetings" to "service_role";

grant trigger on table "public"."greetings" to "service_role";

grant truncate on table "public"."greetings" to "service_role";

grant update on table "public"."greetings" to "service_role";

grant delete on table "public"."locks" to "anon";

grant insert on table "public"."locks" to "anon";

grant references on table "public"."locks" to "anon";

grant select on table "public"."locks" to "anon";

grant trigger on table "public"."locks" to "anon";

grant truncate on table "public"."locks" to "anon";

grant update on table "public"."locks" to "anon";

grant delete on table "public"."locks" to "authenticated";

grant insert on table "public"."locks" to "authenticated";

grant references on table "public"."locks" to "authenticated";

grant select on table "public"."locks" to "authenticated";

grant trigger on table "public"."locks" to "authenticated";

grant truncate on table "public"."locks" to "authenticated";

grant update on table "public"."locks" to "authenticated";

grant delete on table "public"."locks" to "service_role";

grant insert on table "public"."locks" to "service_role";

grant references on table "public"."locks" to "service_role";

grant select on table "public"."locks" to "service_role";

grant trigger on table "public"."locks" to "service_role";

grant truncate on table "public"."locks" to "service_role";

grant update on table "public"."locks" to "service_role";

grant delete on table "public"."notes" to "anon";

grant insert on table "public"."notes" to "anon";

grant references on table "public"."notes" to "anon";

grant select on table "public"."notes" to "anon";

grant trigger on table "public"."notes" to "anon";

grant truncate on table "public"."notes" to "anon";

grant update on table "public"."notes" to "anon";

grant delete on table "public"."notes" to "authenticated";

grant insert on table "public"."notes" to "authenticated";

grant references on table "public"."notes" to "authenticated";

grant select on table "public"."notes" to "authenticated";

grant trigger on table "public"."notes" to "authenticated";

grant truncate on table "public"."notes" to "authenticated";

grant update on table "public"."notes" to "authenticated";

grant delete on table "public"."notes" to "service_role";

grant insert on table "public"."notes" to "service_role";

grant references on table "public"."notes" to "service_role";

grant select on table "public"."notes" to "service_role";

grant trigger on table "public"."notes" to "service_role";

grant truncate on table "public"."notes" to "service_role";

grant update on table "public"."notes" to "service_role";

grant delete on table "public"."notes_settings" to "anon";

grant insert on table "public"."notes_settings" to "anon";

grant references on table "public"."notes_settings" to "anon";

grant select on table "public"."notes_settings" to "anon";

grant trigger on table "public"."notes_settings" to "anon";

grant truncate on table "public"."notes_settings" to "anon";

grant update on table "public"."notes_settings" to "anon";

grant delete on table "public"."notes_settings" to "authenticated";

grant insert on table "public"."notes_settings" to "authenticated";

grant references on table "public"."notes_settings" to "authenticated";

grant select on table "public"."notes_settings" to "authenticated";

grant trigger on table "public"."notes_settings" to "authenticated";

grant truncate on table "public"."notes_settings" to "authenticated";

grant update on table "public"."notes_settings" to "authenticated";

grant delete on table "public"."notes_settings" to "service_role";

grant insert on table "public"."notes_settings" to "service_role";

grant references on table "public"."notes_settings" to "service_role";

grant select on table "public"."notes_settings" to "service_role";

grant trigger on table "public"."notes_settings" to "service_role";

grant truncate on table "public"."notes_settings" to "service_role";

grant update on table "public"."notes_settings" to "service_role";

grant delete on table "public"."pins" to "anon";

grant insert on table "public"."pins" to "anon";

grant references on table "public"."pins" to "anon";

grant select on table "public"."pins" to "anon";

grant trigger on table "public"."pins" to "anon";

grant truncate on table "public"."pins" to "anon";

grant update on table "public"."pins" to "anon";

grant delete on table "public"."pins" to "authenticated";

grant insert on table "public"."pins" to "authenticated";

grant references on table "public"."pins" to "authenticated";

grant select on table "public"."pins" to "authenticated";

grant trigger on table "public"."pins" to "authenticated";

grant truncate on table "public"."pins" to "authenticated";

grant update on table "public"."pins" to "authenticated";

grant delete on table "public"."pins" to "service_role";

grant insert on table "public"."pins" to "service_role";

grant references on table "public"."pins" to "service_role";

grant select on table "public"."pins" to "service_role";

grant trigger on table "public"."pins" to "service_role";

grant truncate on table "public"."pins" to "service_role";

grant update on table "public"."pins" to "service_role";

grant delete on table "public"."report_chat_settings" to "anon";

grant insert on table "public"."report_chat_settings" to "anon";

grant references on table "public"."report_chat_settings" to "anon";

grant select on table "public"."report_chat_settings" to "anon";

grant trigger on table "public"."report_chat_settings" to "anon";

grant truncate on table "public"."report_chat_settings" to "anon";

grant update on table "public"."report_chat_settings" to "anon";

grant delete on table "public"."report_chat_settings" to "authenticated";

grant insert on table "public"."report_chat_settings" to "authenticated";

grant references on table "public"."report_chat_settings" to "authenticated";

grant select on table "public"."report_chat_settings" to "authenticated";

grant trigger on table "public"."report_chat_settings" to "authenticated";

grant truncate on table "public"."report_chat_settings" to "authenticated";

grant update on table "public"."report_chat_settings" to "authenticated";

grant delete on table "public"."report_chat_settings" to "service_role";

grant insert on table "public"."report_chat_settings" to "service_role";

grant references on table "public"."report_chat_settings" to "service_role";

grant select on table "public"."report_chat_settings" to "service_role";

grant trigger on table "public"."report_chat_settings" to "service_role";

grant truncate on table "public"."report_chat_settings" to "service_role";

grant update on table "public"."report_chat_settings" to "service_role";

grant delete on table "public"."report_user_settings" to "anon";

grant insert on table "public"."report_user_settings" to "anon";

grant references on table "public"."report_user_settings" to "anon";

grant select on table "public"."report_user_settings" to "anon";

grant trigger on table "public"."report_user_settings" to "anon";

grant truncate on table "public"."report_user_settings" to "anon";

grant update on table "public"."report_user_settings" to "anon";

grant delete on table "public"."report_user_settings" to "authenticated";

grant insert on table "public"."report_user_settings" to "authenticated";

grant references on table "public"."report_user_settings" to "authenticated";

grant select on table "public"."report_user_settings" to "authenticated";

grant trigger on table "public"."report_user_settings" to "authenticated";

grant truncate on table "public"."report_user_settings" to "authenticated";

grant update on table "public"."report_user_settings" to "authenticated";

grant delete on table "public"."report_user_settings" to "service_role";

grant insert on table "public"."report_user_settings" to "service_role";

grant references on table "public"."report_user_settings" to "service_role";

grant select on table "public"."report_user_settings" to "service_role";

grant trigger on table "public"."report_user_settings" to "service_role";

grant truncate on table "public"."report_user_settings" to "service_role";

grant update on table "public"."report_user_settings" to "service_role";

grant delete on table "public"."rules" to "anon";

grant insert on table "public"."rules" to "anon";

grant references on table "public"."rules" to "anon";

grant select on table "public"."rules" to "anon";

grant trigger on table "public"."rules" to "anon";

grant truncate on table "public"."rules" to "anon";

grant update on table "public"."rules" to "anon";

grant delete on table "public"."rules" to "authenticated";

grant insert on table "public"."rules" to "authenticated";

grant references on table "public"."rules" to "authenticated";

grant select on table "public"."rules" to "authenticated";

grant trigger on table "public"."rules" to "authenticated";

grant truncate on table "public"."rules" to "authenticated";

grant update on table "public"."rules" to "authenticated";

grant delete on table "public"."rules" to "service_role";

grant insert on table "public"."rules" to "service_role";

grant references on table "public"."rules" to "service_role";

grant select on table "public"."rules" to "service_role";

grant trigger on table "public"."rules" to "service_role";

grant truncate on table "public"."rules" to "service_role";

grant update on table "public"."rules" to "service_role";

grant delete on table "public"."users" to "anon";

grant insert on table "public"."users" to "anon";

grant references on table "public"."users" to "anon";

grant select on table "public"."users" to "anon";

grant trigger on table "public"."users" to "anon";

grant truncate on table "public"."users" to "anon";

grant update on table "public"."users" to "anon";

grant delete on table "public"."users" to "authenticated";

grant insert on table "public"."users" to "authenticated";

grant references on table "public"."users" to "authenticated";

grant select on table "public"."users" to "authenticated";

grant trigger on table "public"."users" to "authenticated";

grant truncate on table "public"."users" to "authenticated";

grant update on table "public"."users" to "authenticated";

grant delete on table "public"."users" to "service_role";

grant insert on table "public"."users" to "service_role";

grant references on table "public"."users" to "service_role";

grant select on table "public"."users" to "service_role";

grant trigger on table "public"."users" to "service_role";

grant truncate on table "public"."users" to "service_role";

grant update on table "public"."users" to "service_role";

grant delete on table "public"."warns_settings" to "anon";

grant insert on table "public"."warns_settings" to "anon";

grant references on table "public"."warns_settings" to "anon";

grant select on table "public"."warns_settings" to "anon";

grant trigger on table "public"."warns_settings" to "anon";

grant truncate on table "public"."warns_settings" to "anon";

grant update on table "public"."warns_settings" to "anon";

grant delete on table "public"."warns_settings" to "authenticated";

grant insert on table "public"."warns_settings" to "authenticated";

grant references on table "public"."warns_settings" to "authenticated";

grant select on table "public"."warns_settings" to "authenticated";

grant trigger on table "public"."warns_settings" to "authenticated";

grant truncate on table "public"."warns_settings" to "authenticated";

grant update on table "public"."warns_settings" to "authenticated";

grant delete on table "public"."warns_settings" to "service_role";

grant insert on table "public"."warns_settings" to "service_role";

grant references on table "public"."warns_settings" to "service_role";

grant select on table "public"."warns_settings" to "service_role";

grant trigger on table "public"."warns_settings" to "service_role";

grant truncate on table "public"."warns_settings" to "service_role";

grant update on table "public"."warns_settings" to "service_role";

grant delete on table "public"."warns_users" to "anon";

grant insert on table "public"."warns_users" to "anon";

grant references on table "public"."warns_users" to "anon";

grant select on table "public"."warns_users" to "anon";

grant trigger on table "public"."warns_users" to "anon";

grant truncate on table "public"."warns_users" to "anon";

grant update on table "public"."warns_users" to "anon";

grant delete on table "public"."warns_users" to "authenticated";

grant insert on table "public"."warns_users" to "authenticated";

grant references on table "public"."warns_users" to "authenticated";

grant select on table "public"."warns_users" to "authenticated";

grant trigger on table "public"."warns_users" to "authenticated";

grant truncate on table "public"."warns_users" to "authenticated";

grant update on table "public"."warns_users" to "authenticated";

grant delete on table "public"."warns_users" to "service_role";

grant insert on table "public"."warns_users" to "service_role";

grant references on table "public"."warns_users" to "service_role";

grant select on table "public"."warns_users" to "service_role";

grant trigger on table "public"."warns_users" to "service_role";

grant truncate on table "public"."warns_users" to "service_role";

grant update on table "public"."warns_users" to "service_role";
