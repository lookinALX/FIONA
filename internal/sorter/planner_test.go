package sorter

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanner_AddAction(t *testing.T) {
	sourceFolder := filepath.Join("user", "source")

	action0 := Action{
		SourcePath: filepath.Join(sourceFolder, "file.txt"),
		DestDir:    "documents",
		DestPath:   filepath.Join("home", "dest", "documents"),
		FileSize:   128,
	}

	action1 := Action{
		SourcePath: filepath.Join(sourceFolder, "file1.txt"),
		DestDir:    "documents",
		DestPath:   filepath.Join("home", "dest", "documents"),
		FileSize:   128,
	}

	action2 := Action{
		SourcePath: filepath.Join(sourceFolder, "nestedFolder", "file2.txt"),
		DestDir:    "documents",
		DestPath:   filepath.Join("home", "dest", "documents"),
		FileSize:   128,
	}

	action3 := Action{
		SourcePath: filepath.Join(sourceFolder, "anotherNestedFolder", "file3.txt"),
		DestDir:    "documents",
		DestPath:   filepath.Join("home", "dest", "documents"),
		FileSize:   128,
	}

	deepAction := Action{
		SourcePath: filepath.Join(sourceFolder, "a", "b", "c", "d", "file.txt"),
		FileSize:   128,
		DestDir:    "documents",
	}

	tests := []struct {
		name   string
		plan   Plan
		action Action
		want   Plan
	}{
		{
			name:   "add action to empty plan",
			plan:   Plan{},
			action: action0,
			want: Plan{
				Actions:       []Action{action0},
				BaseDirSource: sourceFolder,
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
		},
		{
			name: "add action to plan with action same source",
			plan: Plan{
				Actions:       []Action{action0},
				BaseDirSource: sourceFolder,
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
			action: action1,
			want: Plan{
				Actions:       []Action{action0, action1},
				BaseDirSource: sourceFolder,
				DirCounts: map[string]int{
					"documents": 2,
				},
				DirSizes: map[string]int64{
					"documents": 128 * 2,
				},
			},
		},
		{
			name: "add action with base dir",
			plan: Plan{
				Actions:       []Action{action2},
				BaseDirSource: filepath.Join(sourceFolder, "nestedFolder"),
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
			action: action0,
			want: Plan{
				Actions:       []Action{action2, action0},
				BaseDirSource: filepath.Join(sourceFolder),
				DirCounts: map[string]int{
					"documents": 2,
				},
				DirSizes: map[string]int64{
					"documents": 128 * 2,
				},
			},
		},
		{
			name: "add action from another nested folder in the same base dir",
			plan: Plan{
				Actions:       []Action{action2},
				BaseDirSource: filepath.Join(sourceFolder, "nestedFolder"),
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
			action: action3,
			want: Plan{
				Actions:       []Action{action2, action3},
				BaseDirSource: sourceFolder,
				DirCounts: map[string]int{
					"documents": 2,
				},
				DirSizes: map[string]int64{
					"documents": 128 * 2,
				},
			},
		},
		{
			name: "add action with no common base dir",
			plan: Plan{
				Actions:       []Action{action0},
				BaseDirSource: filepath.Join("user", "source"),
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
			action: Action{
				SourcePath: filepath.Join("another", "root", "file2.txt"),
				DestDir:    "documents",
				DestPath:   filepath.Join("home", "dest", "documents"),
				FileSize:   128,
			},
			want: Plan{
				Actions: []Action{
					action0,
					{
						SourcePath: filepath.Join("another", "root", "file2.txt"),
						DestDir:    "documents",
						DestPath:   filepath.Join("home", "dest", "documents"),
						FileSize:   128,
					},
				},
				BaseDirSource: ".",
				DirCounts: map[string]int{
					"documents": 2,
				},
				DirSizes: map[string]int64{
					"documents": 128 * 2,
				},
			},
		},
		{
			name: "add deeply nested action",
			plan: Plan{
				Actions:       []Action{action0},
				BaseDirSource: sourceFolder,
			},
			action: deepAction,
			want: Plan{
				Actions:       []Action{action0, deepAction},
				BaseDirSource: sourceFolder,
				DirCounts: map[string]int{
					"documents": 1,
				},
				DirSizes: map[string]int64{
					"documents": 128,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.plan.AddAction(tt.action)
			if got := tt.plan; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Plan.AddAction() = %v, want %v", got, tt.want)
			}
		})
	}
}
