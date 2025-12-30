-- Test: function with dollar quoting and transactional rollback
CREATE FUNCTION test_fn_integration(i integer) RETURNS integer AS $$
BEGIN
  RETURN i + 1;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS tx_postgres_integration (id serial PRIMARY KEY, v integer);
INSERT INTO tx_postgres_integration (v) VALUES (1);

-- Intentional typo to force rollback
INSRT INTO tx_postgres_integration (v) VALUES (2);
