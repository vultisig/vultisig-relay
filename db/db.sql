CREATE DATABASE IF NOT EXISTS voltix;
USE voltix;
Create table if not exists users(
    id int primary key auto_increment,
    api_key varchar(255) not null,
    created_at timestamp default current_timestamp,
    expired_at timestamp null,
    no_of_vaults int default 0,
    is_paid boolean default false
);

Create table if not exists vaults(
    id int primary key auto_increment,
    user_id int not null,
    vault_pubkey_ecdsa varchar(255) not null,
    vault_pubkey_eddsa varchar(255) not null,
    created_at timestamp default current_timestamp
);
Create table if not exists payment_history (
    id int primary key auto_increment,
    user_id int not null,
    amount  DECIMAL(10,2) not null,
    tx_id varchar(255) not null,
    created_at timestamp default current_timestamp
);