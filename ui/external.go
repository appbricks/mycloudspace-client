package ui

type UI interface {
	// returns a handle to a UI message writer
	NewUIMessage(title string) Message
}

// UI defines an interface for interacting with the user
type Message interface {

	// a message which will be formatted for the target platform
	WriteMessage(message string)
	// a generic comment
	WriteCommentMessage(message string)
	// an informational message
	WriteInfoMessage(message string)
	// a note concerning next steps
	WriteNoteMessage(message string)
	// a notification message
	WriteNoticeMessage(message string)
	// an error message
	WriteErrorMessage(message string)
	// a message that indicates a potentially dangerous action
	WriteDangerMessage(message string)
	// a fatal message that will exit the program once displayed
	WriteFatalMessage(message string)

	// write some text without any formatting applied
	WriteText(text string)

	// show messages in the buffer
	ShowMessage()
	// dismiss a message that was previously shown
	DismissMessage()

	// returns a handle to a progress display for a long running task
	ShowMessageWithProgressIndicator(startMsg, progressMsg, endMsg string, doneAt int) ProgressMessage
}

type ProgressMessage interface {
	Start()
	Update(updateMsg string, progressAt int)
	Done()
}
