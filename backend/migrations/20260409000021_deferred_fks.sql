-- Add deferred FK constraints for gov→doc and gov→ai references
-- These columns were created as plain UUIDs; now adding FK constraints
-- since the referenced tables exist.

ALTER TABLE violations 
    ADD CONSTRAINT fk_violations_governing_doc 
    FOREIGN KEY (governing_doc_id) REFERENCES governing_documents(id);

ALTER TABLE arb_requests 
    ADD CONSTRAINT fk_arb_requests_governing_doc 
    FOREIGN KEY (governing_doc_id) REFERENCES governing_documents(id);

ALTER TABLE meetings 
    ADD CONSTRAINT fk_meetings_agenda_doc 
    FOREIGN KEY (agenda_doc_id) REFERENCES documents(id);

ALTER TABLE meetings 
    ADD CONSTRAINT fk_meetings_minutes_doc 
    FOREIGN KEY (minutes_doc_id) REFERENCES documents(id);

ALTER TABLE hearing_links 
    ADD CONSTRAINT fk_hearing_links_notice_doc 
    FOREIGN KEY (notice_doc_id) REFERENCES documents(id);
