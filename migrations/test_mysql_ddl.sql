-- Test: MySQL DDL/DML with semicolons in string and transactional rollback
CREATE TABLE IF NOT EXISTS tx_mysql_integration (id INT AUTO_INCREMENT PRIMARY KEY, note TEXT);
INSERT INTO tx_mysql_integration (note) VALUES ('contains;semicolons;in;string');

-- Intentional typo
INSRT INTO tx_mysql_integration (note) VALUES ('should fail');
