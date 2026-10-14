package utils

import "fmt"

// Step prints message, runs task, then ends the line with "Done! :)" if it
// succeeded. Only plain text and newlines are written (no cursor movement),
// so it reads the same in Windows and Unix-like terminals.
func Step(message string, task func() error) error {
	fmt.Print(message + " ")
	if err := task(); err != nil {
		// Leave the line, so the error starts on its own.
		fmt.Println()
		return err
	}
	fmt.Println("Done! :)")
	return nil
}
