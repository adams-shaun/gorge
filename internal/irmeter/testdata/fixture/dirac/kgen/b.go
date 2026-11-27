package kgen

func refuse(string) {}

func refuseNode(string) {}

func lower() {
	refuse("x")
	refuse("y")
	refuseNode("z")
	_ = "lower case is not forge"
}
