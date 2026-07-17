-- Repair published PostgreSQL search templates.
-- Optional filter parameters are strings, while the source status/jenis columns may
-- be PostgreSQL enums. Comparing the enum directly to '' or a text bind fails.
-- Cast the source enum to text so omitted and supplied filters both bind safely.

UPDATE api_definitions
SET query_template = replace(query_template, 'status = :status', 'status::text = :status')
WHERE path IN (
    '/api/v1/postgres/karyawan/search',
    '/api/v1/postgres/proyek/search',
    '/api/v1/postgres-aws/karyawan/search',
    '/api/v1/postgres-aws/proyek/search'
)
  AND query_template LIKE '%status = :status%';

UPDATE api_definitions
SET query_template = replace(query_template, 't.jenis = :jenis', 't.jenis::text = :jenis')
WHERE path IN (
    '/api/v1/postgres/transaksi/search',
    '/api/v1/postgres-aws/transaksi/search'
)
  AND query_template LIKE '%t.jenis = :jenis%';
