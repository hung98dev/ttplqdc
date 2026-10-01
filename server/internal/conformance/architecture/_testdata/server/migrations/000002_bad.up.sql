ALTER TABLE item_instances ADD COLUMN durability INT NOT NULL DEFAULT 0;
CREATE TABLE global_leader_lease (id TEXT PRIMARY KEY);
