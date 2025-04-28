create database frontdesk;

CREATE SCHEMA adm;
CREATE TABLE adm.users (
	id serial4 NOT NULL,
	username varchar(50) NOT NULL,
	"password" varchar(255) NOT NULL,
	CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE TABLE adm.activesessions (
	userid int4 NOT NULL,
	"token" varchar(255) NOT NULL,
	CONSTRAINT activesessions_pkey PRIMARY KEY (userid)
);
ALTER TABLE adm.activesessions ADD CONSTRAINT activesessions_userid_fkey FOREIGN KEY (userid) REFERENCES adm.users(id);


CREATE SCHEMA paychecker;
create table paychecker.bills (
    id serial primary key, --id is just id
    description varchar(50) not null, --name that bill
    expDay int not null, --expiration day
    lastDate timestamp null, --last payment date
    path varchar(25) not null, --directory for receipts
    track boolean not null default true --track payment
)


CREATE SCHEMA timetracker;
create table timetracker.timesheet (
    id serial primary key
    dt_entry timestamp not null
)