package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openDB(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func OpenDB(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	return openDB(ctx, cfg)
}

func initSchema(ctx context.Context, db *pgxpool.Pool, cfg Config) error {
	ddl := `
create table if not exists users (
  id text primary key,
  username text not null unique,
  display_name text not null,
  password_hash text not null,
  role text not null check (role in ('root','admin','teacher','assistant','student')),
  status text not null check (status in ('active','disabled')),
  created_at timestamptz not null default now()
);

create table if not exists sessions (
  token_hash text primary key,
  user_id text not null references users(id) on delete cascade,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);

create table if not exists courses (
  id text primary key,
  name text not null,
  code text not null unique,
  description text not null default '',
  status text not null check (status in ('active','archived')) default 'active',
  content_root text not null default '',
  created_by text not null references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists course_members (
  course_id text not null references courses(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  member_role text not null check (member_role in ('admin','teacher')),
  joined_at timestamptz not null default now(),
  invited_by text references users(id),
  primary key (course_id, user_id)
);

create table if not exists classes (
  id text primary key,
  course_id text not null,
  name text not null,
  term text not null,
  note text not null default '',
  created_by text not null references users(id),
  created_at timestamptz not null default now()
);

create table if not exists class_assignments (
  id text primary key,
  class_id text not null references classes(id) on delete cascade,
  title text not null,
  open_at timestamptz,
  due_at timestamptz,
  rstudio_image text,
  starter text,
  submit_path text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists class_teachers (
  class_id text not null references classes(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  primary key (class_id, user_id)
);

create table if not exists class_members (
  class_id text not null references classes(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  member_role text not null check (member_role in ('student','assistant')),
  joined_at timestamptz not null default now(),
  primary key (class_id, user_id)
);

create table if not exists submissions (
  id text primary key,
  class_id text not null references classes(id) on delete cascade,
  assignment_id text not null,
  user_id text not null references users(id),
  text_object text not null default '',
  file_object text not null default '',
  attempt integer not null,
  late boolean not null default false,
  created_at timestamptz not null default now()
);

create table if not exists grades (
  id text primary key,
  submission_id text not null unique references submissions(id) on delete cascade,
  score numeric(6,2) not null check (score >= 0 and score <= 100),
  comment text not null default '',
  grader_id text not null references users(id),
  published_at timestamptz,
  updated_at timestamptz not null default now()
);

create table if not exists workspaces (
  id text primary key,
  user_id text not null references users(id),
  class_id text not null references classes(id) on delete cascade,
  assignment_id text not null,
  pod_name text not null,
  service_name text not null,
  pvc_name text not null,
  status text not null,
  last_seen_at timestamptz not null default now(),
  created_at timestamptz not null default now()
);

create table if not exists events (
  id text primary key,
  user_id text not null,
  action text not null,
  target text not null,
  created_at timestamptz not null default now()
);
`
	if _, err := db.Exec(ctx, ddl); err != nil {
		return err
	}

	_, err := db.Exec(ctx, `
insert into users (id, username, display_name, password_hash, role, status)
values ($1, $2, 'Root', $3, 'root', 'active')
on conflict (username) do nothing
`, uuid.NewString(), cfg.RootUsername, cfg.RootPasswordHash)
	if err != nil {
		return err
	}

	// Migrate existing classes → courses
	if _, err := db.Exec(ctx, `
insert into courses (id, name, code, description, created_by)
select distinct on (c.course_id)
  gen_random_uuid()::text,
  c.course_id,
  c.course_id,
  '',
  c.created_by
from classes c
where not exists (select 1 from courses co where co.code = c.course_id)
`); err != nil {
		return err
	}

	// Auto-enroll class creators as course teachers
	if _, err := db.Exec(ctx, `
insert into course_members (course_id, user_id, member_role)
select distinct on (co.id, c.created_by)
  co.id,
  c.created_by,
  'teacher'
from classes c
join courses co on co.code = c.course_id
where not exists (
  select 1 from course_members cm
  where cm.course_id = co.id and cm.user_id = c.created_by
)
`); err != nil {
		return err
	}

	return nil
}

func InitSchema(ctx context.Context, db *pgxpool.Pool, cfg Config) error {
	return initSchema(ctx, db, cfg)
}

func logEvent(ctx context.Context, db *pgxpool.Pool, userID, action, target string) {
	_, _ = db.Exec(ctx, `insert into events (id, user_id, action, target) values ($1,$2,$3,$4)`,
		uuid.NewString(), userID, action, target)
}
