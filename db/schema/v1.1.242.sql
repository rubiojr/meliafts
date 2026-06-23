CREATE TABLE accounts (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        email TEXT NOT NULL,
        type TEXT NOT NULL CHECK(type IN ('imap', 'pop3')),
        incoming_host TEXT NOT NULL,
        incoming_port INTEGER NOT NULL,
        incoming_security TEXT NOT NULL CHECK(incoming_security IN ('ssl', 'starttls', 'none')),
        outgoing_host TEXT NOT NULL,
        outgoing_port INTEGER NOT NULL,
        outgoing_security TEXT NOT NULL CHECK(outgoing_security IN ('ssl', 'starttls', 'none')),
        username TEXT NOT NULL,
        password_encrypted TEXT,
        oauth_provider TEXT CHECK(oauth_provider IN ('google', 'microsoft', NULL)),
        oauth_tokens_encrypted TEXT,
        signature TEXT,
        trash_folder_id TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
      , signature_enabled INTEGER DEFAULT 1, sort_order INTEGER DEFAULT 0, sent_folder_id TEXT, account_is_default INTEGER DEFAULT 0, account_signature_mode TEXT DEFAULT 'new_only', smtp_allow_self_signed INTEGER DEFAULT 0, account_enabled INTEGER DEFAULT 1, signature_html TEXT, account_signature_format TEXT DEFAULT 'plain', account_archive_enabled INTEGER DEFAULT 0, archive_folder_id TEXT, account_archive_mark_read INTEGER DEFAULT 0);
CREATE TABLE folders (
        id TEXT PRIMARY KEY,
        account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
        name TEXT NOT NULL,
        path TEXT NOT NULL,
        type TEXT CHECK(type IN ('inbox', 'sent', 'drafts', 'trash', 'spam', 'custom', NULL)),
        selectable INTEGER DEFAULT 1,
        unread_count INTEGER DEFAULT 0,
        total_count INTEGER DEFAULT 0,
        last_synced DATETIME, special_use TEXT,
        UNIQUE(account_id, path)
      );
CREATE TABLE messages (
        id TEXT PRIMARY KEY,
        account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
        folder_id TEXT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
        message_id TEXT,
        thread_id TEXT,
        from_address TEXT NOT NULL,
        from_name TEXT,
        to_addresses TEXT NOT NULL,
        cc_addresses TEXT,
        bcc_addresses TEXT,
        subject TEXT,
        snippet TEXT,
        body_text TEXT,
        body_html TEXT,
        date DATETIME NOT NULL,
        is_read INTEGER DEFAULT 0,
        is_flagged INTEGER DEFAULT 0,
        has_attachments INTEGER DEFAULT 0,
        attachments TEXT,
        raw_headers TEXT,
        uid INTEGER,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
      , is_draft INTEGER DEFAULT 0, draft_reply_to_id TEXT, draft_reply_type TEXT, auth_trust_level TEXT, auth_results TEXT, auth_client_verified INTEGER DEFAULT 0, is_answered INTEGER DEFAULT 0, is_forwarded INTEGER DEFAULT 0, read_receipt_requested INTEGER DEFAULT 0, read_receipt_sent INTEGER DEFAULT 0, reply_to_addresses TEXT);
CREATE INDEX idx_messages_account_folder ON messages(account_id, folder_id);
CREATE INDEX idx_messages_date ON messages(date DESC);
CREATE INDEX idx_messages_thread ON messages(thread_id);
CREATE INDEX idx_messages_read ON messages(is_read);
CREATE INDEX idx_messages_draft ON messages(is_draft);
CREATE TABLE contacts (
        id TEXT PRIMARY KEY,
        email TEXT NOT NULL UNIQUE,
        name TEXT,
        times_contacted INTEGER DEFAULT 0,
        last_contacted DATETIME,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
      , notes TEXT, contact_vip INTEGER DEFAULT 0);
CREATE INDEX idx_contacts_email ON contacts(email);
CREATE INDEX idx_contacts_name ON contacts(name);
CREATE TABLE "blocked_senders" (
          id TEXT PRIMARY KEY,
          blocked_sender_type TEXT NOT NULL CHECK(blocked_sender_type IN ('email', 'domain', 'pattern')),
          blocked_sender_value TEXT NOT NULL,
          blocked_sender_reason TEXT,
          blocked_sender_actions_taken TEXT,
          blocked_sender_emails_blocked INTEGER DEFAULT 0,
          blocked_sender_date_created DATETIME DEFAULT CURRENT_TIMESTAMP,
          blocked_sender_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
          blocked_sender_enabled INTEGER DEFAULT 1
        );
CREATE UNIQUE INDEX idx_blocked_sender_value
          ON blocked_senders(blocked_sender_type, blocked_sender_value);
CREATE TABLE "trusted_senders" (
          id TEXT PRIMARY KEY,
          trusted_sender_type TEXT NOT NULL CHECK(trusted_sender_type IN ('email', 'domain', 'pattern')),
          trusted_sender_value TEXT NOT NULL,
          trusted_sender_date_created DATETIME DEFAULT CURRENT_TIMESTAMP,
          trusted_sender_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
          trusted_sender_enabled INTEGER DEFAULT 1
        );
CREATE UNIQUE INDEX idx_trusted_sender_value
          ON trusted_senders(trusted_sender_type, trusted_sender_value);
CREATE TABLE sender_stats (
        id TEXT PRIMARY KEY,
        sender_stat_email TEXT NOT NULL,
        sender_stat_domain TEXT NOT NULL,
        sender_stat_emails_received INTEGER DEFAULT 0,
        sender_stat_emails_opened INTEGER DEFAULT 0,
        sender_stat_first_contact DATETIME,
        sender_stat_last_contact DATETIME,
        sender_stat_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP
      );
CREATE UNIQUE INDEX idx_sender_stat_email ON sender_stats(sender_stat_email);
CREATE INDEX idx_sender_stat_domain ON sender_stats(sender_stat_domain);
CREATE TABLE "trust_activity_log" (
          id TEXT PRIMARY KEY,
          activity_action TEXT NOT NULL,
          activity_target_type TEXT CHECK(activity_target_type IN ('email', 'domain', 'pattern')),
          activity_target_value TEXT,
          activity_details TEXT,
          activity_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
        );
CREATE INDEX idx_trust_activity_timestamp
          ON trust_activity_log(activity_timestamp DESC);
CREATE TABLE settings (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL
      );
CREATE TABLE outbox_messages (
        id TEXT PRIMARY KEY,
        outbox_account_id TEXT NOT NULL,
        outbox_to TEXT NOT NULL,
        outbox_cc TEXT,
        outbox_bcc TEXT,
        outbox_subject TEXT,
        outbox_body_html TEXT,
        outbox_body_text TEXT,
        outbox_attachments TEXT,
        outbox_reply_to_message_id TEXT,
        outbox_references TEXT,
        outbox_original_message_db_id TEXT,
        outbox_compose_mode TEXT,
        outbox_status TEXT NOT NULL DEFAULT 'queued' CHECK(outbox_status IN ('queued', 'sending', 'failed')),
        outbox_attempts INTEGER DEFAULT 0,
        outbox_max_attempts INTEGER DEFAULT 3,
        outbox_last_error TEXT,
        outbox_next_retry_at DATETIME,
        outbox_date_created DATETIME DEFAULT CURRENT_TIMESTAMP,
        outbox_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP
      , outbox_priority TEXT DEFAULT NULL, outbox_request_read_receipt INTEGER DEFAULT 0, outbox_request_delivery_receipt INTEGER DEFAULT 0, outbox_alias_id TEXT DEFAULT NULL);
CREATE INDEX idx_outbox_status ON outbox_messages(outbox_status);
CREATE INDEX idx_outbox_account ON outbox_messages(outbox_account_id);
CREATE TABLE deferred_sync_queue (
        id TEXT PRIMARY KEY,
        sync_account_id TEXT NOT NULL,
        sync_folder_id TEXT NOT NULL,
        sync_operation TEXT NOT NULL CHECK(sync_operation IN ('mark_read', 'mark_unread', 'toggle_flag', 'delete_message', 'move_message', 'batch_mark_read', 'batch_mark_unread', 'batch_delete_message')),
        sync_message_uid INTEGER,
        sync_payload TEXT,
        sync_status TEXT NOT NULL DEFAULT 'pending' CHECK(sync_status IN ('pending', 'replaying', 'completed', 'failed')),
        sync_attempts INTEGER DEFAULT 0,
        sync_last_error TEXT,
        sync_date_created DATETIME DEFAULT CURRENT_TIMESTAMP,
        sync_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP
      );
CREATE INDEX idx_deferred_sync_status ON deferred_sync_queue(sync_status);
CREATE INDEX idx_deferred_sync_account ON deferred_sync_queue(sync_account_id);
CREATE INDEX idx_deferred_sync_created ON deferred_sync_queue(sync_date_created);
CREATE TABLE search_history (
        pk_search_history INTEGER PRIMARY KEY AUTOINCREMENT,
        search_history_term TEXT NOT NULL UNIQUE,
        search_history_date_last_used DATETIME DEFAULT CURRENT_TIMESTAMP
      );
CREATE TABLE sqlite_sequence(name,seq);
CREATE TABLE inbox_rules (
          id TEXT PRIMARY KEY,
          rule_account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
          rule_field TEXT NOT NULL CHECK(rule_field IN ('sender_name', 'sender_email', 'sender_domain', 'subject', 'body')),
          rule_operator TEXT NOT NULL CHECK(rule_operator IN ('contains', 'equals', 'matches')),
          rule_value TEXT NOT NULL,
          rule_action TEXT NOT NULL DEFAULT 'spam',
          rule_sort_order INTEGER DEFAULT 0,
          rule_enabled INTEGER DEFAULT 1,
          rule_hits INTEGER DEFAULT 0,
          rule_is_system INTEGER DEFAULT 0,
          rule_date_created DATETIME DEFAULT CURRENT_TIMESTAMP,
          rule_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP
        );
CREATE UNIQUE INDEX idx_inbox_rules_unique
          ON inbox_rules(rule_account_id, rule_field, rule_operator, rule_value);
CREATE INDEX idx_inbox_rules_account
          ON inbox_rules(rule_account_id);
CREATE VIRTUAL TABLE messages_fts USING fts5(
          subject, from_name, from_address, to_text, snippet, body_text,
          content=messages, content_rowid=rowid
        )
/* messages_fts(subject,from_name,from_address,to_text,snippet,body_text) */;
CREATE TABLE 'messages_fts_data'(id INTEGER PRIMARY KEY, block BLOB);
CREATE TABLE 'messages_fts_idx'(segid, term, pgno, PRIMARY KEY(segid, term)) WITHOUT ROWID;
CREATE TABLE 'messages_fts_docsize'(id INTEGER PRIMARY KEY, sz BLOB);
CREATE TABLE 'messages_fts_config'(k PRIMARY KEY, v) WITHOUT ROWID;
CREATE TABLE account_canonical_folders (
          pk_canonical INTEGER PRIMARY KEY AUTOINCREMENT,
          fk_account TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
          canonical_intent TEXT NOT NULL CHECK(canonical_intent IN ('sent', 'trash', 'spam', 'drafts', 'archive')),
          canonical_folder_path TEXT NOT NULL,
          canonical_decided_by TEXT NOT NULL CHECK(canonical_decided_by IN ('auto', 'user')),
          canonical_protected_paths TEXT,
          canonical_date_created DATETIME DEFAULT CURRENT_TIMESTAMP,
          canonical_date_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
          UNIQUE(fk_account, canonical_intent)
        );
CREATE INDEX idx_canonical_account
          ON account_canonical_folders(fk_account);
CREATE TABLE aliases (
          id TEXT PRIMARY KEY,
          account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
          name TEXT NOT NULL,
          email_address TEXT NOT NULL,
          reply_to TEXT,
          signature TEXT,
          created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
          updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        , signature_html TEXT);
CREATE INDEX idx_aliases_account ON aliases(account_id);
CREATE TRIGGER trigger_messages_insert_unread
      AFTER INSERT ON messages WHEN NEW.is_read = 0
      BEGIN
        UPDATE folders SET unread_count = unread_count + 1
        WHERE id = NEW.folder_id
          AND (
            ((type = 'drafts' OR name LIKE '%draft%') AND NEW.is_draft = 1) OR
            ((type IS NULL OR (type != 'drafts' AND name NOT LIKE '%draft%'))
             AND (NEW.is_draft = 0 OR NEW.is_draft IS NULL))
          );
      END;
CREATE TRIGGER trigger_messages_update_read
      AFTER UPDATE OF is_read ON messages WHEN OLD.is_read != NEW.is_read
      BEGIN
        -- Decrement if marked read (was unread, now read)
        UPDATE folders SET unread_count = MAX(0, unread_count - 1)
        WHERE id = NEW.folder_id AND OLD.is_read = 0 AND NEW.is_read = 1
          AND (
            ((type = 'drafts' OR name LIKE '%draft%') AND NEW.is_draft = 1) OR
            ((type IS NULL OR (type != 'drafts' AND name NOT LIKE '%draft%'))
             AND (NEW.is_draft = 0 OR NEW.is_draft IS NULL))
          );

        -- Increment if marked unread (was read, now unread)
        UPDATE folders SET unread_count = unread_count + 1
        WHERE id = NEW.folder_id AND OLD.is_read = 1 AND NEW.is_read = 0
          AND (
            ((type = 'drafts' OR name LIKE '%draft%') AND NEW.is_draft = 1) OR
            ((type IS NULL OR (type != 'drafts' AND name NOT LIKE '%draft%'))
             AND (NEW.is_draft = 0 OR NEW.is_draft IS NULL))
          );
      END;
CREATE TRIGGER trigger_messages_delete_unread
      AFTER DELETE ON messages WHEN OLD.is_read = 0
      BEGIN
        UPDATE folders SET unread_count = MAX(0, unread_count - 1)
        WHERE id = OLD.folder_id
          AND (
            ((type = 'drafts' OR name LIKE '%draft%') AND OLD.is_draft = 1) OR
            ((type IS NULL OR (type != 'drafts' AND name NOT LIKE '%draft%'))
             AND (OLD.is_draft = 0 OR OLD.is_draft IS NULL))
          );
      END;
CREATE TRIGGER trigger_messages_fts_insert
      AFTER INSERT ON messages
      BEGIN
        INSERT INTO messages_fts(rowid, subject, from_name, from_address, to_text, snippet, body_text)
        VALUES (NEW.rowid, NEW.subject, NEW.from_name, NEW.from_address, 
      TRIM(
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(NEW.to_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(NEW.cc_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(NEW.bcc_addresses, '[]'))
        ), '')
      )
    , NEW.snippet, NEW.body_text);
      END;
CREATE TRIGGER trigger_messages_fts_delete
      AFTER DELETE ON messages
      BEGIN
        INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_address, to_text, snippet, body_text)
        VALUES ('delete', OLD.rowid, OLD.subject, OLD.from_name, OLD.from_address, 
      TRIM(
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(OLD.to_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(OLD.cc_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(OLD.bcc_addresses, '[]'))
        ), '')
      )
    , OLD.snippet, OLD.body_text);
      END;
CREATE TRIGGER trigger_messages_fts_update
      AFTER UPDATE OF subject, from_name, from_address, to_addresses, cc_addresses, bcc_addresses, snippet, body_text ON messages
      BEGIN
        INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_address, to_text, snippet, body_text)
        VALUES ('delete', OLD.rowid, OLD.subject, OLD.from_name, OLD.from_address, 
      TRIM(
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(OLD.to_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(OLD.cc_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(OLD.bcc_addresses, '[]'))
        ), '')
      )
    , OLD.snippet, OLD.body_text);
        INSERT INTO messages_fts(rowid, subject, from_name, from_address, to_text, snippet, body_text)
        VALUES (NEW.rowid, NEW.subject, NEW.from_name, NEW.from_address, 
      TRIM(
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(NEW.to_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(NEW.cc_addresses, '[]'))
        ), '') || ' ' ||
        IFNULL((SELECT GROUP_CONCAT(
          COALESCE(json_extract(value, '$.name'), '') || ' ' ||
          COALESCE(json_extract(value, '$.address'), ''), ' ')
          FROM json_each(IFNULL(NEW.bcc_addresses, '[]'))
        ), '')
      )
    , NEW.snippet, NEW.body_text);
      END;
