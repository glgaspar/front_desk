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
    expire timestamp not null,
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
    created varchar(255), -- thats temporary, dont worry
    status varchar(255), -- thats temporary, dont worry
    exitcode varchar(255), -- thats temporary, dont worry
    error varchar(255), -- thats temporary, dont worry
    startedat varchar(255), -- thats temporary, dont worry
    finishedat varchar(255), -- thats temporary, dont worry
    image varchar(255), -- thats temporary, dont worry
    name varchar(255), -- thats temporary, dont worry
    restartcount varchar(255), -- thats temporary, dont worry
    labels varchar(255), -- thats temporary, dont worry
    project varchar(255), -- thats temporary, dont worry
    configfiles varchar(255), -- thats temporary, dont worry
    workingdir varchar(255), -- thats temporary, dont worry
    replace varchar(255), -- thats temporary, dont worry
    link varchar(255) -- thats temporary, dont worry
)
