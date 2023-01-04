package ipsets

import (
	"strings"
)

const pathSeparator = "|"

type IPSets struct {
	Sets    *stringStore
	Entries *stringStore
}

func New() IPSets {
	return IPSets{
		Sets:    newStringStore(),
		Entries: newStringStore(),
	}
}

// Done calls each store's Done. See diffstore.Store#Done
func (s IPSets) Done() {
	s.Sets.Done()
	s.Entries.Done()
}

// Reset calls each store's Reset. See diffstore.Store#Reset.
func (s IPSets) Reset() {
	s.Sets.Reset()
	s.Entries.Reset()
}

func (s IPSets) AddEntry(set string, entry string) {
	s.Sets.Get(set).Set(set)
	s.Entries.Get(set + pathSeparator + entry).Set(entry)
}

type Entry struct {
	Set   string
	Entry string
}

func (s IPSets) Created() (sets []string, entries []Entry) {
	changedSets := s.Sets.Changed()
	sets = make([]string, 0, len(changedSets))

	for _, item := range changedSets {
		if item.Created() {
			set := item.Value().Get()
			sets = append(sets, set)
		}
	}

	changedEntries := s.Entries.Changed()
	entries = make([]Entry, 0, len(changedEntries))

	for _, item := range changedEntries {
		if item.Created() {
			set, _, _ := strings.Cut(item.Key(), pathSeparator)

			entries = append(entries, Entry{
				Set:   set,
				Entry: item.Value().Get(),
			})
		}
	}

	return
}

func (s IPSets) Deleted() (sets []string, entries []Entry) {
	deletedSets := s.Sets.Deleted()
	sets = make([]string, 0, len(deletedSets))

	for _, item := range deletedSets {
		set := item.Value().Get()
		sets = append(sets, set)
	}

	// func to check if a set was deleted
	isSetDeleted := func(a string) bool {
		for _, b := range sets {
			if a == b {
				return true
			}
		}
		return false
	}

	deletedEntries := s.Entries.Deleted()
	entries = make([]Entry, 0, len(deletedEntries))

	for _, item := range deletedEntries {
		set, _, _ := strings.Cut(item.Key(), pathSeparator)

		// filter out entries in sets that are already removed, as they'll be flushed
		if isSetDeleted(set) {
			continue
		}

		entries = append(entries, Entry{
			Set:   set,
			Entry: item.Value().Get(),
		})
	}

	return
}
