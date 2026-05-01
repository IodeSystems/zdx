CREATE INDEX zdx_issues_node_ref_idx
  ON zdx_issues(project_id, node_ref)
  WHERE node_ref IS NOT NULL AND node_ref != '';
