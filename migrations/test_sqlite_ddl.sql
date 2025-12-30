-- Test: SQLite DDL/DML with semicolons inside strings and transactional rollback
CREATE TABLE IF NOT EXISTS tx_sqlite_integration (id INTEGER PRIMARY KEY, txt TEXT);
INSERT INTO tx_sqlite_integration (txt) VALUES ('value;with;semis');

-- Intentional typo
INSRT INTO tx_sqlite_integration (txt) VALUES ('this fails');
