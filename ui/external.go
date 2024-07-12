package ui

import "context"

type UI interface {
	// returns a handle to a UI message writer
	NewUIMessage(title string) Message
	// returns a handle to a cancellable UI message writer
	NewUIMessageWithCancel(title string, cancel context.CancelFunc) Message
	
	// shows an error message
	ShowErrorMessage(message string)
	// shows an information message
	ShowInfoMessage(title, message string)
	// shows an note message
	ShowNoteMessage(title, message string)
	// shows an notice message
	ShowNoticeMessage(title, message string)
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

	// show message with user input
	ShowMessageWithInput(defaultInput string, handleInput func(*string))	
	ShowMessageWithSecureInput(handleInput func(*string))
	ShowMessageWithSecureVerifiedInput(handleInput func(*string))
	ShowMessageWithYesNoInput(handleInput func(bool))
	ShowMessageWithFileInput(handleInput func(*string))

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
