package routines

import (
	"harmony/backend/model"
	"time"
)

// calls the correct handler based on the contents of the first message.
func MasterTestRoutine(fromCl <-chan string, toCl chan<- string, c *model.Client) {
	// inspect the first message to send it to the right handler
	firstMsg := <-fromCl

	if firstMsg == "Hello\n" {
		comeOnline(fromCl, toCl, c)
	} else if firstMsg == "Test\n" {
		testRoutine(fromCl, toCl)
	} else if firstMsg == "WaitForAges\n" {
		waitForAges(toCl)
	} else {
		toCl <- "Unrecognized routine"
	}

}

func testRoutine(fromCl <-chan string, toCl chan<- string) {

	toCl <- "Please type your name\n"
	name := <-fromCl
	toCl <- "Hello, " + name + "!\n"
	toCl <- "What is your favourite food?\n"
	food := <-fromCl
	toCl <- "I love " + food + "!\n"
	toCl <- "Goodbye for now!\n"

}

func comeOnline(fromCl <-chan string, toCl chan<- string, c *model.Client) {

	if c.PublicKey != nil {
		toCl <- "You are already online"
		return
	}

	toCl <- "My version number is..."
	<-fromCl
	// publicKey := <-fromCl
	// *c.PublicKey = ([model.KEYLEN]byte)(publicKey)
	toCl <- "Welcome"

}

func waitForAges(toCl chan<- string) {

	toCl <- "Hm, let me just think for a bit..."
	time.Sleep(20 * time.Second)
	toCl <- "Ok bye"

}
