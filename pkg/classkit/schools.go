package classkit

// Schools are the magic disciplines a class can belong to. Physical classes
// use martial. The id is the value stored in YAML (`school:`); the display
// name is for the client. Keep this list small and stable: a class whose
// school is not registered fails to load.
var schoolNames = map[string]string{
	"martial": "Martial",
	"fire":    "Fire",
	"ice":     "Ice",
	"arcane":  "Arcane",
	"divine":  "Divine",
	"nature":  "Nature",
	"shadow":  "Shadow",
}

// SchoolName returns the display name for a school id. Unknown or empty ids
// return "".
func SchoolName(id string) string {
	return schoolNames[norm(id)]
}

// KnownSchool reports whether id is a registered school. The empty string is
// not a school; a class may omit `school` and fall back to a physical kit.
func KnownSchool(id string) bool {
	_, ok := schoolNames[norm(id)]
	return ok
}
