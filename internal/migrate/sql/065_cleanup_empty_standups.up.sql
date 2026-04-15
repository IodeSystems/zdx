DELETE FROM zdx_journal_entries
WHERE tldr LIKE 'Auto-generated%'
  AND assessment = ''
  AND concerns = ''
  AND next = '';
