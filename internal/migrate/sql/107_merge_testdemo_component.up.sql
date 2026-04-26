-- Merge duplicate zdx_tests rows where TestDemo* tests were registered under
-- component='api' (old client behavior) AND component='demo' (current behavior).
-- For solo 'api' rows with TestDemo* names, rename them to 'demo'.

DO $$
BEGIN

-- Pairs: same (project_id, name) exists under both 'api' and 'demo'.
CREATE TEMP TABLE _testdemo_dup_pairs AS
SELECT api_t.id AS api_id, demo_t.id AS demo_id
FROM zdx_tests api_t
JOIN zdx_tests demo_t
    ON demo_t.project_id = api_t.project_id
   AND demo_t.name       = api_t.name
   AND demo_t.component  = 'demo'
WHERE api_t.component = 'api'
  AND api_t.name LIKE 'TestDemo%';

-- spec_tests: drop api refs that would conflict, then migrate the rest.
DELETE FROM zdx_spec_tests st
USING _testdemo_dup_pairs dp
WHERE st.test_id = dp.api_id
  AND EXISTS (
    SELECT 1 FROM zdx_spec_tests x WHERE x.spec_id = st.spec_id AND x.test_id = dp.demo_id
  );

UPDATE zdx_spec_tests SET test_id = dp.demo_id
FROM _testdemo_dup_pairs dp
WHERE zdx_spec_tests.test_id = dp.api_id;

-- test_demos: drop api refs that would conflict, then migrate the rest.
DELETE FROM zdx_test_demos td
USING _testdemo_dup_pairs dp
WHERE td.test_id = dp.api_id
  AND EXISTS (
    SELECT 1 FROM zdx_test_demos x
    WHERE x.test_id      = dp.demo_id
      AND x.demo_type    = td.demo_type
      AND x.artifact_path = td.artifact_path
  );

UPDATE zdx_test_demos SET test_id = dp.demo_id
FROM _testdemo_dup_pairs dp
WHERE zdx_test_demos.test_id = dp.api_id;

-- test_code_refs: drop api refs that would conflict, then migrate the rest.
DELETE FROM zdx_test_code_refs cr
USING _testdemo_dup_pairs dp
WHERE cr.test_id = dp.api_id
  AND EXISTS (
    SELECT 1 FROM zdx_test_code_refs x WHERE x.test_id = dp.demo_id AND x.code_ref_id = cr.code_ref_id
  );

UPDATE zdx_test_code_refs SET test_id = dp.demo_id
FROM _testdemo_dup_pairs dp
WHERE zdx_test_code_refs.test_id = dp.api_id;

-- Delete the now-orphaned 'api' rows (RESTRICT FKs are cleared above).
DELETE FROM zdx_tests WHERE id IN (SELECT api_id FROM _testdemo_dup_pairs);

DROP TABLE _testdemo_dup_pairs;

-- Solo 'api' rows for TestDemo* with no matching 'demo' sibling: rename to 'demo'.
UPDATE zdx_tests SET component = 'demo'
WHERE component = 'api' AND name LIKE 'TestDemo%';

END $$;
