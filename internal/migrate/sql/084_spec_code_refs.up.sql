CREATE TABLE zdx_spec_code_refs (
    spec_id     INT NOT NULL REFERENCES zdx_specs(id)     ON DELETE CASCADE,
    code_ref_id INT NOT NULL REFERENCES zdx_code_refs(id) ON DELETE CASCADE,
    PRIMARY KEY (spec_id, code_ref_id)
);
CREATE INDEX idx_spec_code_refs_spec ON zdx_spec_code_refs (spec_id);
