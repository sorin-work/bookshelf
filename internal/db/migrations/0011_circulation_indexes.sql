-- The dashboard counts today's checkouts and returns and the activity
-- feed sorts by both; without these every dashboard load scanned loans.
CREATE INDEX loans_checked_out_at ON loans (checked_out_at);
CREATE INDEX loans_returned_at ON loans (returned_at) WHERE returned_at IS NOT NULL;
CREATE INDEX fines_open ON fines (status) WHERE status = 'open';
