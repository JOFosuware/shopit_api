ALTER TABLE cloudinary_cleanup_jobs
    ADD COLUMN cleanup_ready BOOLEAN NOT NULL DEFAULT TRUE;

-- Existing jobs may be staged uploads from the previous expiry-based design.
-- Leave them pending until they can be reconciled safely.
UPDATE cloudinary_cleanup_jobs
SET cleanup_ready = FALSE;
