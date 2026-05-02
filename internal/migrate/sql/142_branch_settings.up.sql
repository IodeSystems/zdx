ALTER TABLE zdx_version_branches
    ADD COLUMN merge_style text CHECK (merge_style IN ('squash', 'merge', 'rebase')),
    ADD COLUMN required_checks text;
