package main

// calls the correct handler based on the contents of the first message.
func MasterRoutine(fromCl <-chan string, toCl chan<- string, c *Client) {
	// inspect the first message to send it to the right handler
	firstMsg := <-fromCl

	if firstMsg == "Hello\n" {
		comeOnline(fromCl, toCl, c)
	} else if firstMsg == "Test\n" {
		testRoutine(fromCl, toCl)
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

func comeOnline(fromCl <-chan string, toCl chan<- string, c *Client) {

	if c.publicKey != "" {
		toCl <- "You are already online"
		return
	}

	toCl <- "My version number is..."
	publicKey := <-fromCl
	c.publicKey = publicKey
	toCl <- "Welcome"

}
