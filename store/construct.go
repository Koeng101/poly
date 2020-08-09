package main

import (
	"bytes"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func (x *Index) exportSa() []int64 {
	var sa []int64
	sa = x.sa.int64
	if sa == nil {
		for _, saint32 := range x.sa.int32 {
			sa = append(sa, int64(saint32))
		}
	}
	return sa
}

func (x *Index) exportLcp() []int64 {
	x.makeLcpOnce()
	var lcp []int64
	lcp = x.lcp.int64
	if lcp == nil {
		for _, lcpint32 := range x.lcp.int32 {
			lcp = append(lcp, int64(lcpint32))
		}
	}
	return lcp
}

func initSaDatabase(dbFile string) error {
	db, err := sql.Open("sqlite3", dbFile)
	checkErr(err)

	tx, err := db.Begin()
	checkErr(err)
	defer db.Close()

	_, err = tx.Exec(`
-- Create sequence table
CREATE TABLE sequence(
        seqhash TEXT PRIMARY KEY,
        sequence TEXT NOT NULL,
        circular BOOLEAN NOT NULL,
        sequence_type TEXT NOT NULL CHECK(sequence_type IN ('protein','dna','rna'))
);
CREATE INDEX idx_sequence ON sequence(seqhash,sequence_type);

CREATE TABLE sequence_reference(
        id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT,
        unique_identifier TEXT UNIQUE,

        created_on DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_on DATETIME DEFAULT CURRENT_TIMESTAMP,

        seqhash_id TEXT NOT NULL REFERENCES sequence(seqhash),
        translation TEXT REFERENCES sequence(seqhash),
        organization TEXT,
        database TEXT,

        annotations json

);
CREATE INDEX idx_sequence_reference ON sequence_reference (seqhash_id,translation,organization,database,unique_identifier);

CREATE TRIGGER sequence_reference_update AFTER UPDATE ON sequence_reference
 BEGIN
  update sequence_reference SET updated_on = datetime('now') WHERE id = NEW.id;
 END;


-- Suffix arrays
CREATE TABLE suffix_array_sequence(
        position INTEGER PRIMARY KEY,
        letter TEXT NOT NULL
);

CREATE TABLE sequence_positions(
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        seqhash_id TEXT REFERENCES sequence(seqhash),
        seq_start INTEGER REFERENCES suffix_array_sequence(position),
        seq_end INTEGER REFERENCES suffix_array_sequence(position)
);
CREATE INDEX idx_sequence_positions on sequence_positions (seqhash_id,seq_start,seq_end);

CREATE TABLE suffix_array(
        position INTEGER PRIMARY KEY,
        num INTEGER REFERENCES suffix_array_sequence(position),
        lcp INTEGER
);
CREATE INDEX idx_suffix_array on suffix_array(position,num,lcp);
        `)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

type SequenceStruct struct {
	name               string
	seqhash            string
	sequence           string
	circular           bool
	sequenceType       string
	translationSeqhash string
	translation        string
	annotation         string
}

func insertSeqDatabase(dbFile string, data []SequenceStruct) error {
	db, err := sql.Open("sqlite3", dbFile)
	checkErr(err)
	tx, err := db.Begin()
	checkErr(err)
	defer db.Close()

	for _, d := range data {
		// If no seqhash exists, insert it
		seqhashInsert, _ := tx.Prepare("INSERT INTO sequence(seqhash, sequence, circular, sequence_type) VALUES (?,?,?,?) ON CONFLICT DO NOTHING")
		_, err = seqhashInsert.Exec(d.seqhash, d.sequence, d.circular, d.sequenceType)
		if err != nil {
			tx.Rollback()
			return err
		}
		// If there is a translation, and no seqhash exists, insert it
		if d.translation != "" {
			seqhashTranslationInsert, _ := tx.Prepare("INSERT INTO sequence(seqhash, sequence, circular, sequence_type) VALUES (?,?,?,?) ON CONFLICT DO NOTHING")
			_, err = seqhashTranslationInsert.Exec(d.translationSeqhash, d.translation, false, "protein")
			if err != nil {
				tx.Rollback()
				return err
			}
		}
		// Insert sequence references
		if d.translation == "" {
			noTranslationInsert, _ := tx.Prepare("INSERT INTO sequence_reference(name, seqhash_id, annotations) VALUES (?,?,?)")
			_, err = noTranslationInsert.Exec(d.name, d.seqhash, d.annotation)
		} else {
			translationInsert, _ := tx.Prepare("INSERT INTO sequence_reference(name, seqhash_id, annotations, translation) VALUES (?,?,?,?)")
			_, err = translationInsert.Exec(d.name, d.seqhash, d.annotation, d.translationSeqhash)
		}
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

func insertSaDatabase(dbFile string) error {
	seqhashes, concatSeq := dbString(dbFile)
	p := []byte{}
	var b []byte
	for _, full_seq := range concatSeq {
		b = []byte(full_seq)
		p = append(p, b...)
	}
	na := New(p)
	sa := na.exportSa()
	lcp := na.exportLcp()

	db, err := sql.Open("sqlite3", dbFile)
	checkErr(err)
	tx, err := db.Begin()
	checkErr(err)
	defer db.Close()
	_, err = tx.Exec(`
	DELETE FROM suffix_array_sequence;
	DELETE FROM sequence_positions;
	DELETE FROM suffix_array;
	`)

	// Simply adding to a string here will not work
	var buffer bytes.Buffer
	for _, se := range concatSeq {
		buffer.WriteString(se)
	}

	sa_seq_stmt, err := tx.Prepare("INSERT INTO suffix_array_sequence(position,letter) VALUES (?,?)")
	for i, s := range buffer.String() {
		_, err = sa_seq_stmt.Exec(i, string(s))
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	start := 0
	sh_stmt, err := tx.Prepare("INSERT INTO sequence_positions(seqhash_id,seq_start,seq_end) VALUES (?,?,?)")
	for i, sh := range seqhashes {
		_, err = sh_stmt.Exec(sh, start, start+len(concatSeq[i]))
		if err != nil {
			tx.Rollback()
			return err
		}
		start = start + len(concatSeq[i])
	}

	stmt, err := tx.Prepare("INSERT INTO suffix_array(position,num,lcp) VALUES (?,?,?)")
	for i, d := range sa {
		_, err = stmt.Exec(i, d, lcp[i])
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

func dbString(dbFile string) ([]string, []string) {
	db, err := sql.Open("sqlite3", dbFile)
	checkErr(err)
	rows, _ := db.Query("SELECT sequence as seq, seqhash as seqhash FROM sequence;")
	defer rows.Close()
	var seq string
	var seqhash string
	seqs := []string{}
	seqhashes := []string{}
	for rows.Next() {
		rows.Scan(&seq, &seqhash)
		seqs = append(seqs, seq)
		seqhashes = append(seqhashes, seqhash)
	}
	return seqhashes, seqs
}

func main() {
	// Init db
	t := "test.db"
	err := initSaDatabase(t)
	if err != nil {
		fmt.Println(err)
	}
	// Fill db with the good stuff
	puc19 := SequenceStruct{"pUC19", "4b0616d1b3fc632e42d78521deb38b44fba95cca9fde159e01cd567fa996ceb9",
		"gagatacctacagcgtgagctatgagaaagcgccacgcttcccgaagggagaaaggcggacaggtatccggtaagcggcagggtcggaacaggagagcgcacgagggagcttccagggggaaacgcctggtatctttatagtcctgtcgggtttcgccacctctgacttgagcgtcgatttttgtgatgctcgtcaggggggcggagcctatggaaaaacgccagcaacgcggcctttttacggttcctggccttttgctggccttttgctcacatgttctttcctgcgttatcccctgattctgtggataaccgtattaccgcctttgagtgagctgataccgctcgccgcagccgaacgaccgagcgcagcgagtcagtgagcgaggaagcggaagagcgcccaatacgcaaaccgcctctccccgcgcgttggccgattcattaatgcagctggcacgacaggtttcccgactggaaagcgggcagtgagcgcaacgcaattaatgtgagttagctcactcattaggcaccccaggctttacactttatgcttccggctcgtatgttgtgtggaattgtgagcggataacaatttcacacaggaaacagctatgaccatgattacgccaagcttgcatgcctgcaggtcgactctagaggatccccgggtaccgagctcgaattcactggccgtcgttttacaacgtcgtgactgggaaaaccctggcgttacccaacttaatcgccttgcagcacatccccctttcgccagctggcgtaatagcgaagaggcccgcaccgatcgcccttcccaacagttgcgcagcctgaatggcgaatggcgcctgatgcggtattttctccttacgcatctgtgcggtatttcacaccgcatatggtgcactctcagtacaatctgctctgatgccgcatagttaagccagccccgacacccgccaacacccgctgacgcgccctgacgggcttgtctgctcccggcatccgcttacagacaagctgtgaccgtctccgggagctgcatgtgtcagaggttttcaccgtcatcaccgaaacgcgcgagacgaaagggcctcgtgatacgcctatttttataggttaatgtcatgataataatggtttcttagacgtcaggtggcacttttcggggaaatgtgcgcggaacccctatttgtttatttttctaaatacattcaaatatgtatccgctcatgagacaataaccctgataaatgcttcaataatattgaaaaaggaagagtatgagtattcaacatttccgtgtcgcccttattcccttttttgcggcattttgccttcctgtttttgctcacccagaaacgctggtgaaagtaaaagatgctgaagatcagttgggtgcacgagtgggttacatcgaactggatctcaacagcggtaagatccttgagagttttcgccccgaagaacgttttccaatgatgagcacttttaaagttctgctatgtggcgcggtattatcccgtattgacgccgggcaagagcaactcggtcgccgcatacactattctcagaatgacttggttgagtactcaccagtcacagaaaagcatcttacggatggcatgacagtaagagaattatgcagtgctgccataaccatgagtgataacactgcggccaacttacttctgacaacgatcggaggaccgaaggagctaaccgcttttttgcacaacatgggggatcatgtaactcgccttgatcgttgggaaccggagctgaatgaagccataccaaacgacgagcgtgacaccacgatgcctgtagcaatggcaacaacgttgcgcaaactattaactggcgaactacttactctagcttcccggcaacaattaatagactggatggaggcggataaagttgcaggaccacttctgcgctcggcccttccggctggctggtttattgctgataaatctggagccggtgagcgtgggtctcgcggtatcattgcagcactggggccagatggtaagccctcccgtatcgtagttatctacacgacggggagtcaggcaactatggatgaacgaaatagacagatcgctgagataggtgcctcactgattaagcattggtaactgtcagaccaagtttactcatatatactttagattgatttaaaacttcatttttaatttaaaaggatctaggtgaagatcctttttgataatctcatgaccaaaatcccttaacgtgagttttcgttccactgagcgtcagaccccgtagaaaagatcaaaggatcttcttgagatcctttttttctgcgcgtaatctgctgcttgcaaacaaaaaaaccaccgctaccagcggtggtttgtttgccggatcaagagctaccaactctttttccgaaggtaactggcttcagcagagcgcagataccaaatactgttcttctagtgtagccgtagttaggccaccacttcaagaactctgtagcaccgcctacatacctcgctctgctaatcctgttaccagtggctgctgccagtggcgataagtcgtgtcttaccgggttggactcaagacgatagttaccggataaggcgcagcggtcgggctgaacggggggttcgtgcacacagcccagcttggagcgaacgacctacaccgaact",
		true, "dna", "", "", "The pUC19 plasmid"}
	lacZ := SequenceStruct{"lacZ", "496b64f287ddb9caf8dbb6bff59c1548d264faa60f6577d9a83d37238a5a8711",
		"atgaccatgattacgccaagcttgcatgcctgcaggtcgactctagaggatccccgggtaccgagctcgaattcactggccgtcgttttacaacgtcgtgactgggaaaaccctggcgttacccaacttaatcgccttgcagcacatccccctttcgccagctggcgtaatagcgaagaggcccgcaccgatcgcccttcccaacagttgcgcagcctgaatggcgaatggcgcctgatgcggtattttctccttacgcatctgtgcggtatttcacaccgcatatggtgcactctcagtacaatctgctctgatgccgcatag",
		false, "dna", "85e550ebf8540c573d13115b3789e0d01ef72195e3100968b2fa4f2b60a12c65", "MTMITPSLHACRSTLEDPRVPSSNSLAVVLQRRDWENPGVTQLNRLAAHPPFASWRNSEEARTDRPSQQLRSLNGEWRLMRYFLLTHLCGISHRIWCTLSTICSDAA", "lacZ fragment"}
	err = insertSeqDatabase(t, []SequenceStruct{puc19, lacZ})
	// Build out that sa and lcp array
	err = insertSaDatabase(t)
	if err != nil {
		fmt.Println(err)
	}
}
