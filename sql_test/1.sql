create table if not exists migra_test (
	id serial primary key,
	name varchar(255) not null,
	created_at timestamptz not null default now()
);
