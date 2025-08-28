-- Active: 1756348506129@@192.168.10.11@5432@frontdesk
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
    id serial primary key,
    dt_entry timestamp not null
)

create SCHEMA apps
create table apps.list (
    id varchar(255) PRIMARY key,
    command varchar(255), -- thats temporary, dont worry
    createdat varchar(255), -- thats temporary, dont worry
    image varchar(255), -- thats temporary, dont worry
    labels varchar(255), -- thats temporary, dont worry
    localvolumes varchar(255), -- thats temporary, dont worry
    mounts varchar(255), -- thats temporary, dont worry
    names varchar(255), -- thats temporary, dont worry
    networks varchar(255), -- thats temporary, dont worry
    ports varchar(255), -- thats temporary, dont worry
    runningfor varchar(255), -- thats temporary, dont worry
    size varchar(255), -- thats temporary, dont worry
    state varchar(255), -- thats temporary, dont worry
    status varchar(255), -- thats temporary, dont worry
    link varchar(255) -- thats temporary, dont worry
)
