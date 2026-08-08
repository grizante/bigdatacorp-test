// Package pipeline moves records from a JSONL stream to two CSV streams.
//
// It owns every decision to skip a record and every diagnostic that decision
// produces; package domain owns what the surviving values mean.
package pipeline

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"bigdatacorp-test/domain"
)

// maxLineBytes caps a single input line. A club with a large players array
// exceeds bufio.Scanner's 64 KB default, which would otherwise end the run
// mid-file.
const maxLineBytes = 1 << 20 // 1 MiB

// initialBufBytes is the scanner's starting buffer; it grows up to maxLineBytes
// as needed, so ordinary lines never pay for the cap.
const initialBufBytes = 64 * 1024

// Run streams in, writing club rows to clubsOut, player rows to playersOut, and
// diagnostics about skipped records to logw.
//
// It takes writers rather than paths so that callers — main and the tests
// alike — decide where the bytes land; logw is injectable specifically so the
// "logged versus silent" distinction can be asserted on.
//
// Both CSVs always receive their header, even when no record survives. A
// malformed line is skipped and the run continues; only a read or write failure
// returns an error, and rows written before it are flushed regardless.
func Run(in io.Reader, clubsOut, playersOut, logw io.Writer) error {
	clubs := csv.NewWriter(clubsOut)
	players := csv.NewWriter(playersOut)

	// RFC 4180 terminates rows with CRLF, but csv.Writer defaults to a bare LF.
	// It has to be asked for.
	clubs.UseCRLF = true
	players.UseCRLF = true

	// Flush on every exit path, so a mid-stream failure still leaves the rows
	// that had already been accepted.
	defer clubs.Flush()
	defer players.Flush()

	if err := clubs.Write(domain.ClubHeader()); err != nil {
		return fmt.Errorf("writing clubs header: %w", err)
	}
	if err := players.Write(domain.PlayerHeader()); err != nil {
		return fmt.Errorf("writing players header: %w", err)
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, initialBufBytes), maxLineBytes)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Blank line: nothing to report. This inspects the raw line to decide
		// whether to read it at all; it is not field normalization, which
		// happens only in domain.Normalize.
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var club domain.Club
		if err := json.Unmarshal(line, &club); err != nil {
			fmt.Fprintf(logw, "line %d: invalid JSON: %v\n", lineNum, err)
			continue
		}

		club.Normalize()

		// The filter runs first, and its skip is silent unconditionally: a club
		// we were never going to export produces no diagnostics however broken
		// the rest of it is. Logging is reserved for data we wanted.
		if !club.IsExportable() {
			continue
		}
		if !club.HasID() {
			fmt.Fprintf(logw, "line %d: skipping club: missing club_id\n", lineNum)
			continue
		}

		if err := clubs.Write(club.Record()); err != nil {
			return fmt.Errorf("writing club %q: %w", club.ClubID, err)
		}

		for i := range club.Players {
			player := &club.Players[i]
			if !player.IsExportable() {
				fmt.Fprintf(logw, "line %d: skipping player %d of club %s: missing player_id\n",
					lineNum, i+1, club.ClubID)
				continue
			}
			if err := players.Write(player.Record(club.ClubID)); err != nil {
				return fmt.Errorf("writing player %q: %w", player.PlayerID, err)
			}
		}
	}

	clubs.Flush()
	players.Flush()

	// Reported after the flush: the scan stops for good at this point, so the
	// accepted rows are on their way out before the failure is raised.
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read failed at line %d: %w", lineNum+1, err)
	}
	if err := clubs.Error(); err != nil {
		return fmt.Errorf("writing clubs.csv: %w", err)
	}
	if err := players.Error(); err != nil {
		return fmt.Errorf("writing players.csv: %w", err)
	}
	return nil
}
