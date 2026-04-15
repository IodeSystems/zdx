CREATE TABLE zdx_test_code_refs (
    test_id    INT  NOT NULL REFERENCES zdx_tests(id)     ON DELETE CASCADE,
    code_ref_id INT NOT NULL REFERENCES zdx_code_refs(id) ON DELETE CASCADE,
    PRIMARY KEY (test_id, code_ref_id)
);
CREATE INDEX idx_test_code_refs_test ON zdx_test_code_refs (test_id);
