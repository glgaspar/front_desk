-- create database frontdesk;

CREATE SCHEMA IF NOT EXISTS adm;
CREATE TABLE IF NOT EXISTS adm.users (
	id serial4 NOT NULL,
	username varchar(50) NOT NULL,
	password varchar(255) NOT NULL,
	CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS adm.activesessions (
	userid int4 NOT NULL,
	token varchar(255) NOT NULL,
    expire timestamp not null,
	CONSTRAINT activesessions_pkey PRIMARY KEY (token)
);
ALTER TABLE adm.activesessions ADD CONSTRAINT IF NOT EXISTS activesessions_userid_fkey FOREIGN KEY (userid) REFERENCES adm.users(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS adm.cloudflare (
    config_id int4 NOT NULL DEFAULT 1,
    accountId varchar(255) NOT NULL,
    tunnelId varchar(255) NOT NULL,
    cloudflareAPIToken varchar(255) NOT NULL,
    localAddress varchar(255) NOT NULL,
    zoneId varchar(255) NOT NULL,
    CONSTRAINT cloudflare_pkey PRIMARY KEY (config_id),
    CONSTRAINT cloudflare_single_row CHECK (config_id = 1)
);