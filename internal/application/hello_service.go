package application

type ApplicationService struct {
}

func (a *ApplicationService) GenerateHello(name string) string {
	return "Hello, " + name + " !"
}
