create database frontdesk;

CREATE SCHEMA adm;
CREATE TABLE adm.users (
	id serial NOT NULL,
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

create schema taskmaster;
create table if not exists taskmaster.status (
    id serial primary key,
    name varchar(50) not null,
    color varchar(7) not null
);

create table if not exists taskmaster.workspace (
    id serial primary key,
    name varchar(50) not null,
    description text,
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatedAt TIMESTAMP NULL
);

create table if not exists taskmaster.task (
    id serial primary key,
    title varchar(55) not null,
    idUserCreated int not null FOREIGN KEY REFERENCES adm.users(id),
    description text,
    assignee int not null FOREIGN KEY REFERENCES adm.users(id), 
    idStatus int not null FOREIGN KEY REFERENCES taskmaster.status(id),
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatedAt TIMESTAMP NULL,
    deadline   TIMESTAMP NULL,
    idWorkspace int not null FOREIGN KEY REFERENCES taskmaster.workspace(id) ON DELETE CASCADE,
);

create table if not exists taskmaster.tag (
    id serial primary key,
    name varchar(50) not null,
    color varchar(7) not null
);

create table if not exists taskmaster.comment (
    id serial primary key,
    idTask int not null FOREIGN KEY REFERENCES taskmaster.tasks(id) ON DELETE CASCADE,
    idUser int not null FOREIGN KEY REFERENCES adm.users(id),
    content text not null,
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

create table if not exists taskmaster.subtask (
    idTask int not null FOREIGN KEY REFERENCES taskmaster.task(id) ON DELETE CASCADE,
    id serial primary key,
    title varchar(55) not null,
    idUserCreated int not null FOREIGN KEY REFERENCES adm.users(id),
    description text,
    assignee int not null FOREIGN KEY REFERENCES adm.users(id), 
    idStatus int not null FOREIGN KEY REFERENCES taskmaster.status(id),
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatedAt TIMESTAMP NULL,
    deadline   TIMESTAMP NULL,
    idWorkspace int not null FOREIGN KEY REFERENCES taskmaster.workspace(id) ON DELETE CASCADE
);