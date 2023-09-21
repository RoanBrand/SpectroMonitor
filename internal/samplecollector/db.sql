-- sudo -u postgres psql -f sql/init.sql

-- create database if it doesn't exist
SELECT 'CREATE DATABASE testsamplesdb'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'testsamplesdb')\gexec

\c "testsamplesdb"

-- create user for service
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT FROM pg_catalog.pg_roles
    WHERE rolname = 'samplecollectorservice') THEN

    CREATE ROLE samplecollectorservice LOGIN PASSWORD 'notrealpass';
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS "test_samples" (
    "id" BIGSERIAL PRIMARY KEY,
    "test_time" TIMESTAMP WITH TIME ZONE NOT NULL,
    "spectro_machine" INT NOT NULL,
    "furnace_name" TEXT NOT NULL,
    "sample_name" TEXT
);
CREATE INDEX IF NOT EXISTS test_samples_time_idx ON test_samples(test_time);
CREATE UNIQUE INDEX IF NOT EXISTS test_samples_idx ON test_samples(test_time, spectro_machine, LOWER(furnace_name));
ALTER TABLE "test_samples" OWNER TO "samplecollectorservice";

CREATE TABLE IF NOT EXISTS "sample_results" (
    "id" BIGINT PRIMARY KEY REFERENCES test_samples(id),

    "C" DOUBLE PRECISION NOT NULL,
    "Si" DOUBLE PRECISION NOT NULL,
    "Mn" DOUBLE PRECISION NOT NULL,
    "P" DOUBLE PRECISION NOT NULL,
    "S" DOUBLE PRECISION NOT NULL,
    "Cu" DOUBLE PRECISION NOT NULL,
    "Cr" DOUBLE PRECISION NOT NULL,
    "Al" DOUBLE PRECISION NOT NULL,
    "Ti" DOUBLE PRECISION NOT NULL,
    "Sn" DOUBLE PRECISION NOT NULL,
    "Zn" DOUBLE PRECISION NOT NULL,
    "Pb" DOUBLE PRECISION NOT NULL,

    "Ni" DOUBLE PRECISION,
    "Mo" DOUBLE PRECISION,
    "Co" DOUBLE PRECISION,
    "Nb" DOUBLE PRECISION,
    "V" DOUBLE PRECISION,
    "Mo" DOUBLE PRECISION,
    "W" DOUBLE PRECISION,
    "Mg" DOUBLE PRECISION,
    "Bi" DOUBLE PRECISION,
    "Ca" DOUBLE PRECISION,
    "As" DOUBLE PRECISION,
    "Sb" DOUBLE PRECISION,
    "Te" DOUBLE PRECISION,
    "Fe" DOUBLE PRECISION
);
ALTER TABLE "sample_results" OWNER TO "samplecollectorservice";