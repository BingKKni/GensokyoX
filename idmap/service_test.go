package idmap

import (
	"encoding/binary"
	"path/filepath"
	"strconv"
	"testing"

	"go.etcd.io/bbolt"
)

func openTestDB(t *testing.T) {
	t.Helper()

	oldDB := db
	testDB, err := bbolt.Open(filepath.Join(t.TempDir(), DBName), 0600, nil)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := testDB.Update(func(tx *bbolt.Tx) error {
		for _, name := range []string{BucketName, CacheBucketName, ConfigBucket, UserInfoBucket} {
			if _, err := tx.CreateBucket([]byte(name)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		testDB.Close()
		t.Fatalf("initialize test database: %v", err)
	}
	db = testDB
	t.Cleanup(func() {
		_ = testDB.Close()
		db = oldDB
	})
}

func bucketCounter(t *testing.T, bucketName string) uint64 {
	t.Helper()
	var counter uint64
	if err := db.View(func(tx *bbolt.Tx) error {
		value := tx.Bucket([]byte(bucketName)).Get([]byte(CounterKey))
		if len(value) == 8 {
			counter = binary.BigEndian.Uint64(value)
		}
		return nil
	}); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	return counter
}

func TestStoreIDNeverOverwritesReverseMapping(t *testing.T) {
	openTestDB(t)

	first, err := StoreID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil || first != 1 {
		t.Fatalf("first mapping = %d, %v; want 1", first, err)
	}
	second, err := StoreID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil || second != 2 {
		t.Fatalf("second mapping = %d, %v; want 2", second, err)
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		counter := make([]byte, 8)
		return tx.Bucket([]byte(BucketName)).Put([]byte(CounterKey), counter)
	}); err != nil {
		t.Fatalf("lower counter: %v", err)
	}

	third, err := StoreID("cccccccccccccccccccccccccccccccc")
	if err != nil || third != 3 {
		t.Fatalf("third mapping = %d, %v; want 3", third, err)
	}
	real, err := RetrieveRowByID("1")
	if err != nil || real != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("row 1 changed to %q, %v", real, err)
	}
}

func TestStoreCacheEmptyDoesNotAdvanceCounter(t *testing.T) {
	openTestDB(t)

	row, err := StoreCache("")
	if err != nil || row != 0 {
		t.Fatalf("StoreCache(empty) = %d, %v; want 0, nil", row, err)
	}
	if got := bucketCounter(t, CacheBucketName); got != 0 {
		t.Fatalf("cache counter = %d; want 0", got)
	}
}

func TestCleanBucketRemovesOnlyTemporaryEventsAndPreservesCounter(t *testing.T) {
	openTestDB(t)

	stableID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	eventID := "GROUP_MEMBER_ADD:11111111-2222-3333-4444-555555555555"
	for _, id := range []string{stableID, eventID, "1234567"} {
		if _, err := StoreID(id); err != nil {
			t.Fatalf("store %q: %v", id, err)
		}
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(BucketName)).Delete([]byte(CounterKey))
	}); err != nil {
		t.Fatalf("delete counter: %v", err)
	}

	deleted, migrated, err := cleanBucket(db, BucketName)
	if err != nil {
		t.Fatalf("clean bucket: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted entries = %d; want 2", deleted)
	}
	if migrated != 0 {
		t.Fatalf("migrated entries = %d; want 0", migrated)
	}
	if got := bucketCounter(t, BucketName); got != 3 {
		t.Fatalf("restored counter = %d; want 3", got)
	}
	if _, err := RetrieveRowByID("2"); err != ErrKeyNotFound {
		t.Fatalf("temporary reverse mapping still exists: %v", err)
	}
	if real, err := RetrieveRowByID("1"); err != nil || real != stableID {
		t.Fatalf("stable mapping changed to %q, %v", real, err)
	}
	if real, err := RetrieveRowByID("3"); err != nil || real != "1234567" {
		t.Fatalf("numeric mapping changed to %q, %v", real, err)
	}

	next, err := StoreID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil || next != 4 {
		t.Fatalf("next mapping = %d, %v; want 4", next, err)
	}
}

func TestClearCacheDoesNotReuseShortIDs(t *testing.T) {
	openTestDB(t)

	for _, id := range []string{"message-1", "message-2"} {
		if _, err := StoreCache(id); err != nil {
			t.Fatalf("store cache %q: %v", id, err)
		}
	}
	if err := clearBucket(db, CacheBucketName); err != nil {
		t.Fatalf("clear cache: %v", err)
	}
	if _, err := RetrieveRowByCache("1"); err != ErrKeyNotFound {
		t.Fatalf("cleared row 1 still exists: %v", err)
	}

	next, err := StoreCache("message-3")
	if err != nil || next != 3 {
		t.Fatalf("next cache mapping = %d, %v; want 3", next, err)
	}
}

func TestCompactionRequiresFreshTargetAndCopiesMappings(t *testing.T) {
	openTestDB(t)

	if _, err := StoreID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("store mapping: %v", err)
	}
	target := filepath.Join(t.TempDir(), "compacted.db")
	if err := Compaction(db.Path(), target); err != nil {
		t.Fatalf("compact database: %v", err)
	}
	if err := Compaction(db.Path(), target); err == nil {
		t.Fatal("second compaction unexpectedly reused an existing target")
	}

	compacted, err := bbolt.Open(target, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatalf("open compacted database: %v", err)
	}
	defer compacted.Close()
	if err := compacted.View(func(tx *bbolt.Tx) error {
		value := tx.Bucket([]byte(BucketName)).Get([]byte("row-1"))
		if string(value) != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
			t.Fatalf("compacted row 1 = %q", value)
		}
		return nil
	}); err != nil {
		t.Fatalf("read compacted database: %v", err)
	}
}

func TestStoreIDSkipsViolateID(t *testing.T) {
	openTestDB(t)

	if err := db.Update(func(tx *bbolt.Tx) error {
		counter := make([]byte, 8)
		binary.BigEndian.PutUint64(counter, 8963)
		return tx.Bucket([]byte(BucketName)).Put([]byte(CounterKey), counter)
	}); err != nil {
		t.Fatalf("set counter: %v", err)
	}

	row, err := StoreID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil || row != 8965 {
		t.Fatalf("allocated row = %d, %v; want 8965", row, err)
	}
	if _, err := RetrieveRowByID("8964"); err != ErrKeyNotFound {
		t.Fatalf("reserved row 8964 was allocated: %v", err)
	}
}

func TestStoreIDRemapsExistingViolateID(t *testing.T) {
	openTestDB(t)
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketName))
		row := make([]byte, 8)
		binary.BigEndian.PutUint64(row, 8964)
		if err := bucket.Put([]byte(id), row); err != nil {
			return err
		}
		if err := bucket.Put([]byte("row-8964"), []byte(id)); err != nil {
			return err
		}
		counter := make([]byte, 8)
		binary.BigEndian.PutUint64(counter, 10000)
		return bucket.Put([]byte(CounterKey), counter)
	}); err != nil {
		t.Fatalf("seed reserved mapping: %v", err)
	}

	row, err := StoreID(id)
	if err != nil || row != 10001 {
		t.Fatalf("remapped row = %d, %v; want 10001", row, err)
	}
	if _, err := RetrieveRowByID("8964"); err != ErrKeyNotFound {
		t.Fatalf("old reserved reverse mapping still exists: %v", err)
	}
	if real, err := RetrieveRowByID("10001"); err != nil || real != id {
		t.Fatalf("new reverse mapping = %q, %v", real, err)
	}
}

func TestCleanBucketRemapsAllViolateIDs(t *testing.T) {
	openTestDB(t)
	ids := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	rows := []uint64{8964, 189640}
	if err := db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketName))
		for index, id := range ids {
			rowBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(rowBytes, rows[index])
			if err := bucket.Put([]byte(id), rowBytes); err != nil {
				return err
			}
			if err := bucket.Put([]byte("row-"+strconv.FormatUint(rows[index], 10)), []byte(id)); err != nil {
				return err
			}
		}
		counter := make([]byte, 8)
		binary.BigEndian.PutUint64(counter, 200000)
		return bucket.Put([]byte(CounterKey), counter)
	}); err != nil {
		t.Fatalf("seed reserved mappings: %v", err)
	}

	deleted, migrated, err := cleanBucket(db, BucketName)
	if err != nil {
		t.Fatalf("clean bucket: %v", err)
	}
	if deleted != 0 || migrated != 2 {
		t.Fatalf("clean result = deleted %d, migrated %d; want 0, 2", deleted, migrated)
	}
	for _, oldRow := range rows {
		if _, err := RetrieveRowByID(strconv.FormatUint(oldRow, 10)); err != ErrKeyNotFound {
			t.Fatalf("reserved row %d still exists: %v", oldRow, err)
		}
	}
	for _, id := range ids {
		_, virtual, err := RetrieveVirtualValue(id)
		if err != nil {
			t.Fatalf("retrieve remapped ID %q: %v", id, err)
		}
		row, err := strconv.ParseInt(virtual, 10, 64)
		if err != nil || IsViolateID(row) {
			t.Fatalf("replacement virtual ID %q is invalid or violate", virtual)
		}
	}
}
