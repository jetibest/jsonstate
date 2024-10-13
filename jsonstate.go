package jsonstate

import "fmt"
import "strings"

const (
	StateUnknown int = 0 // includes 'loading', 'not-applicable'
	StateDisabled int = 100 // manually disabled
	StateOk int = 200 // may include optimization hint or info message
	StateAttention int = 300 // something may go wrong in the future
	StateWarning int = 400 // something went wrong, but has little effect (on core functionality)
	StateError int = 500 // something went wrong, and has effect on core functionality, but may automatically recover
	StateFault int = 600 // something is going wrong, and cannot be automatically recovered, manual intervention required
	StatePanic int = 700 // something is going wrong (crash), and it's uncertain what the consequences are, so the worst must be assumed, and manual intervention is always required
)

// note: Tree is an array so that we may set a custom logical order, but "source" should be unique for each State object in the same Tree!
// note: we may store a /etc/<module>/state_override.json file with custom levels, and then override with s.Apply(importedState)
// note: if source is empty, we semantically refer to the parent State
// note: fetch state with /state/ which returns a json-file with the root state of that module, any public API for State does not have a setter, because the API server must monitor the components
//       if components update the state themselves, then we'd at least need a timing mechanism, that automatically invalidates the state after X seconds of no update

type State struct {
	Level int          `json:"level"`
	Source string      `json:"source,omitempty"`
	Message string     `json:"message,omitempty"`
	Tree []*State      `json:"tree,omitempty"`
	Override bool      `json:"override,omitempty"`
}
type FlatState struct {
	Depth int          `json:"depth"`
	Level int          `json:"level"`
	Source string      `json:"source,omitempty"`
	Message string     `json:"message,omitempty"`
	Override bool      `json:"override,omitempty"`
}

// constructor: jsonstate.New(...) instead of &jsonstate.State{}, the former is slightly more readable though less flexible
func New(source string) *State {
	return &State{
		Source: source,
	}
}
func LevelString(level int) string {
	
	if level < 100 {
		
		return "Unknown"
		
	} else if level < 200 {
		
		return "Disabled"
		
	} else if level < 300 {
		
		return "OK"
		
	} else if level < 400 {
		
		return "Attention"
		
	} else if level < 500 {
		
		return "Warning"
		
	} else if level < 600 {
		
		return "Error"
		
	} else if level < 700 {
		
		return "Fault"
		
	} else {
		
		return "Panic"
		
	}
}

// apply override state object recursively, it will never introduce new states though, that would be confusing, because then something may become a tree, where it is not supposed to be as such
func (s *State) Apply(override *State) {
	if override == nil {
		return // nothing to apply
	}
	
	// override Level and Message iff Source matches
	if override.Source != s.Source {
		s.Override = true
		s.Level = override.Level
		s.Message = override.Message
	}
	
	if override.Tree != nil && s.Tree != nil {
		
		for _, override_it := range override.Tree {
			
			// apply override to the entire tree of s, if a wildcard is specified
			list := s.Tree
			
			// if no wildcard is specified, filter by the exact source (including empty Source exact matching)
			if override_it.Source != "*" {
				list = []*State{}
				if s.Tree != nil {
					for _, s_it := range s.Tree {
						if s_it.Source == override_it.Source {
							list = append(list, s_it)
						}
					}
				}
			}
			
			// apply to every filtered tree
			for _, s_it := range list {
				s_it.Apply(override_it)
			}
		}
	}
}
// this also means, no Tree can/should exist (the root state must be re-evaluated if any level is changed, with rootState.AggregateLevels())
func (s *State) Set(level int, message string) *State {
	s.Level = level
	s.Message = message
	
	return s
}
// add new state to tree
func (s *State) Add(s_list ...*State) *State {
	
	for _, s_it := range s_list {
		s.Tree = append(s.Tree, s_it)
	}
	
	return s
}
// return matching sources (recurse for multiple parameters)
func (s *State) FindBySource(source_path ...string) *State {
	
	if s.Tree == nil {
		return nil
	}
	if len(source_path) > 0 {
		
		source := source_path[0]
		
		for _, s_it := range s.Tree {
			if s_it.Source == source {
				
				if len(source_path) > 1 {
					return s_it.FindBySource(source_path[1:]...)
				} else {
					return s_it
				}
			}
		}
	}
	
	return nil
}
// aggregate levels in this State's recursive tree
func (s *State) AggregateLevels() *State {
	
	if s.Tree == nil {
		return s
	}
	
	maxLevel := 0
	for _, s_it := range s.Tree {
		
		// update s_it.Level with the aggregated level
		s_it.AggregateLevels()
		
		if s_it.Level > maxLevel {
			maxLevel = s_it.Level
		}
	}
	
	s.Level = maxLevel
	
	return s
}
// this is particularly useful for exporting to a flat list for simple iteration
func (s *State) Flatten() []*FlatState {
	return rflat(s, 0)
}
// human readable string (one should probably call AggregateLevels() first)
func (s *State) String() string {
	
	var sb strings.Builder
	
	for _, item := range s.Flatten() {
		
		for i := 0; i < item.Depth; i += 1 {
			sb.WriteString("  ")
		}
		
		if item.Source != "" {
			sb.WriteString(fmt.Sprintf("- [%s]: ", item.Source))
		}
		
		sb.WriteString(fmt.Sprintf("%d %s", item.Level, LevelString(item.Level)))
		
		if item.Message != "" {
			sb.WriteString(": ")
			sb.WriteString(item.Message)
		}
		
		sb.WriteString("\n")
	}
	
	return sb.String()
}

func rflat(rs *State, depth int) []*FlatState {
	
	list := []*FlatState{}
	
	list = append(list, &FlatState{
		Depth: depth,
		Level: rs.Level,
		Source: rs.Source,
		Message: rs.Message,
	})
	
	if rs.Tree != nil {
		
		for _, rs_it := range rs.Tree {
			
			for _, rss := range rflat(rs_it, depth + 1) {
				
				list = append(list, rss)
			}
		}
	}
	
	return list
}
