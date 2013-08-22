package server

type Instance interface {
	Name() string                                                // read-only string for the server's name, e.g. "mopidy"
	Start(configPath string) error                               // start it
	Connect(hostname, port string, stop chan bool) (bool, error) // connect to it
	Errors() <-chan error                                        // read-only channel of errors that the server reports
}
