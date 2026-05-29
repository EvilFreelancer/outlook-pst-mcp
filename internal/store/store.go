package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Folder struct {
	ID       string
	ParentID *string
	Name     string
	Path     string
}

type Message struct {
	ID        string
	FolderID  string
	Subject   string
	FromAddr  string
	ToAddrs   []string
	CcAddrs   []string
	EMLPath   string
	Deleted   bool
	MessageAt int64
}

type MessagePatch struct {
	Subject  *string
	FromAddr *string
	ToAddrs  *([]string)
	CcAddrs  *([]string)
	EMLPath  *string
	Deleted  *bool
}

type MessageFilter struct {
	FolderID       string
	Query          string
	Limit          int
	Offset         int
	IncludeDeleted bool
}

type Change struct {
	ID        int64
	MessageID string
	Operation string
	Payload   string
	CreatedAt time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	st := &Store{db: db}
	if err := st.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func sqliteDSN(path string) string {
	const params = "_busy_timeout=5000&_journal_mode=WAL"
	if strings.Contains(path, "?") {
		return path + "&" + params
	}
	return path + "?" + params
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateFolder(name, path string, parentID *string) (Folder, error) {
	id := newID("fld")
	_, err := s.db.Exec(`insert into folders(id,parent_id,name,path,created_at) values(?,?,?,?,?)`, id, parentID, name, path, time.Now().UTC())
	if err != nil {
		return Folder{}, err
	}
	return Folder{ID: id, ParentID: parentID, Name: name, Path: path}, nil
}

func (s *Store) GetFolderByPath(path string) (Folder, bool, error) {
	row := s.db.QueryRow(`select id,parent_id,name,path from folders where path=?`, path)
	folder, err := scanFolder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, false, nil
	}
	if err != nil {
		return Folder{}, false, err
	}
	return folder, true, nil
}

func (s *Store) ListFolders() ([]Folder, error) {
	rows, err := s.db.Query(`select id,parent_id,name,path from folders order by path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []Folder
	for rows.Next() {
		folder, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, rows.Err()
}

func (s *Store) CreateMessage(message Message) (Message, error) {
	if message.MessageAt == 0 {
		message.MessageAt = time.Now().Unix()
	}
	if message.ID == "" {
		id, err := s.allocateMessageID(message.MessageAt)
		if err != nil {
			return Message{}, err
		}
		message.ID = id
	}
	if err := s.withTx(func(tx *sql.Tx) error {
		if err := insertMessage(tx, message); err != nil {
			return err
		}
		return insertChange(tx, message.ID, "create", message)
	}); err != nil {
		return Message{}, err
	}
	return message, nil
}

func (s *Store) InsertMessages(messages []Message) error {
	if len(messages) == 0 {
		return nil
	}
	return s.withTx(func(tx *sql.Tx) error {
		for _, message := range messages {
			if err := insertMessage(tx, message); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) GetMessage(id string) (Message, bool, error) {
	row := s.db.QueryRow(`select id,folder_id,subject,from_addr,to_addrs,cc_addrs,eml_path,deleted,message_at from messages where id=?`, id)
	message, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, false, nil
	}
	if err != nil {
		return Message{}, false, err
	}
	return message, true, nil
}

func (s *Store) UpdateMessage(id string, patch MessagePatch) error {
	message, found, err := s.GetMessage(id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("message %q not found", id)
	}
	if patch.Subject != nil {
		message.Subject = *patch.Subject
	}
	if patch.FromAddr != nil {
		message.FromAddr = *patch.FromAddr
	}
	if patch.ToAddrs != nil {
		message.ToAddrs = append([]string(nil), (*patch.ToAddrs)...)
	}
	if patch.CcAddrs != nil {
		message.CcAddrs = append([]string(nil), (*patch.CcAddrs)...)
	}
	if patch.EMLPath != nil {
		message.EMLPath = *patch.EMLPath
	}
	if patch.Deleted != nil {
		message.Deleted = *patch.Deleted
	}
	return s.withTx(func(tx *sql.Tx) error {
		if err := updateMessage(tx, message); err != nil {
			return err
		}
		return insertChange(tx, id, "update", patch)
	})
}

func (s *Store) MoveMessage(id, folderID string) error {
	return s.withTx(func(tx *sql.Tx) error {
		result, err := tx.Exec(`update messages set folder_id=?,updated_at=? where id=?`, folderID, time.Now().UTC(), id)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return fmt.Errorf("message %q not found", id)
		}
		return insertChange(tx, id, "move", map[string]string{"folder_id": folderID})
	})
}

func (s *Store) DeleteMessage(id string) error {
	deleted := true
	return s.withTx(func(tx *sql.Tx) error {
		result, err := tx.Exec(`update messages set deleted=1,updated_at=? where id=?`, time.Now().UTC(), id)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return fmt.Errorf("message %q not found", id)
		}
		return insertChange(tx, id, "delete", map[string]bool{"deleted": deleted})
	})
}

func (s *Store) ListMessages(filter MessageFilter) ([]Message, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	args := []any{}
	where := "where 1=1"
	if !filter.IncludeDeleted {
		where += " and deleted=0"
	}
	if filter.FolderID != "" {
		where += " and folder_id=?"
		args = append(args, filter.FolderID)
	}
	if filter.Query != "" {
		where += " and (lower(subject) like lower(?) or lower(from_addr) like lower(?) or lower(to_addrs) like lower(?))"
		q := "%" + filter.Query + "%"
		args = append(args, q, q, q)
	}
	var total int
	if err := s.db.QueryRow(`select count(*) from messages `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, filter.Offset)
	rows, err := s.db.Query(`select id,folder_id,subject,from_addr,to_addrs,cc_addrs,eml_path,deleted,message_at from messages `+where+` order by message_at asc, id asc limit ? offset ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, message)
	}
	return messages, total, rows.Err()
}

func (s *Store) ListChanges() ([]Change, error) {
	rows, err := s.db.Query(`select id,message_id,operation,payload,created_at from changes order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var changes []Change
	for rows.Next() {
		var change Change
		var created string
		if err := rows.Scan(&change.ID, &change.MessageID, &change.Operation, &change.Payload, &created); err != nil {
			return nil, err
		}
		change.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
create table if not exists folders(
  id text primary key,
  parent_id text,
  name text not null,
  path text not null unique,
  created_at text not null
);
create table if not exists messages(
  id text primary key,
  folder_id text not null,
  subject text not null,
  from_addr text not null,
  to_addrs text not null,
  cc_addrs text not null,
  eml_path text not null,
  deleted integer not null default 0,
  message_at integer not null default 0,
  created_at text not null,
  updated_at text not null
);
create table if not exists changes(
  id integer primary key autoincrement,
  message_id text,
  operation text not null,
  payload text not null,
  created_at text not null
);`)
	return err
}

func (s *Store) withTx(fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func insertMessage(tx *sql.Tx, message Message) error {
	toJSON, err := json.Marshal(message.ToAddrs)
	if err != nil {
		return err
	}
	ccJSON, err := json.Marshal(message.CcAddrs)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`insert into messages(id,folder_id,subject,from_addr,to_addrs,cc_addrs,eml_path,deleted,message_at,created_at,updated_at) values(?,?,?,?,?,?,?,?,?,?,?)`,
		message.ID, message.FolderID, message.Subject, message.FromAddr, string(toJSON), string(ccJSON), message.EMLPath, boolInt(message.Deleted), message.MessageAt, now, now)
	return err
}

func updateMessage(tx *sql.Tx, message Message) error {
	toJSON, err := json.Marshal(message.ToAddrs)
	if err != nil {
		return err
	}
	ccJSON, err := json.Marshal(message.CcAddrs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`update messages set subject=?,from_addr=?,to_addrs=?,cc_addrs=?,eml_path=?,deleted=?,updated_at=? where id=?`,
		message.Subject, message.FromAddr, string(toJSON), string(ccJSON), message.EMLPath, boolInt(message.Deleted), time.Now().UTC().Format(time.RFC3339Nano), message.ID)
	return err
}

func insertChange(tx *sql.Tx, messageID, operation string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`insert into changes(message_id,operation,payload,created_at) values(?,?,?,?)`, messageID, operation, string(data), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFolder(row rowScanner) (Folder, error) {
	var folder Folder
	var parent sql.NullString
	if err := row.Scan(&folder.ID, &parent, &folder.Name, &folder.Path); err != nil {
		return Folder{}, err
	}
	if parent.Valid {
		folder.ParentID = &parent.String
	}
	return folder, nil
}

func scanMessage(row rowScanner) (Message, error) {
	var message Message
	var toJSON, ccJSON string
	var deleted int
	if err := row.Scan(&message.ID, &message.FolderID, &message.Subject, &message.FromAddr, &toJSON, &ccJSON, &message.EMLPath, &deleted, &message.MessageAt); err != nil {
		return Message{}, err
	}
	_ = json.Unmarshal([]byte(toJSON), &message.ToAddrs)
	_ = json.Unmarshal([]byte(ccJSON), &message.CcAddrs)
	message.Deleted = deleted == 1
	return message, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func (s *Store) allocateMessageID(messageAt int64) (string, error) {
	for suffix := 0; ; suffix++ {
		id := strconv.FormatInt(messageAt, 10)
		if suffix > 0 {
			id = fmt.Sprintf("%d_%d", messageAt, suffix)
		}
		var exists int
		err := s.db.QueryRow(`select 1 from messages where id=?`, id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", err
		}
	}
}
