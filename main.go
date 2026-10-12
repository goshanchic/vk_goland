package main

import (
	"fmt"
	"strings"
)

// Item represents an item in a room or inventory.
type Item struct {
	Name     string
	Place    string // e.g. "на столе", "на стуле"
	Wearable bool
}

// InteractiveObject represents an object the player can apply items to.
type InteractiveObject struct {
	Name    string
	OnApply func(item string) (string, bool)
}

// Transition represents an exit to another room with an optional condition (e.g. locked door).
type Transition struct {
	RoomName string
	Room     *Room
	CanPass  func() (bool, string)
}

// Room represents a room in the game world.
type Room struct {
	Name        string
	Items       []*Item
	Objects     []*InteractiveObject
	Transitions []*Transition
	LookFunc    func(r *Room, p *Player) string
	EnterFunc   func(r *Room) string
}

func (r *Room) AvailablePaths() string {
	names := make([]string, len(r.Transitions))
	for i, t := range r.Transitions {
		names[i] = t.RoomName
	}
	return strings.Join(names, ", ")
}

func (r *Room) GetTransition(name string) *Transition {
	for _, t := range r.Transitions {
		if t.RoomName == name {
			return t
		}
	}
	return nil
}

func (r *Room) GetObject(name string) *InteractiveObject {
	for _, obj := range r.Objects {
		if obj.Name == name {
			return obj
		}
	}
	return nil
}

func (r *Room) FindItem(name string) *Item {
	for _, item := range r.Items {
		if item.Name == name {
			return item
		}
	}
	return nil
}

func (r *Room) RemoveItem(name string) *Item {
	for i, item := range r.Items {
		if item.Name == name {
			r.Items = append(r.Items[:i], r.Items[i+1:]...)
			return item
		}
	}
	return nil
}

func (r *Room) Look(p *Player) string {
	if r.LookFunc != nil {
		return r.LookFunc(r, p)
	}
	return "ничего интересного. можно пройти - " + r.AvailablePaths()
}

func (r *Room) Enter() string {
	if r.EnterFunc != nil {
		return r.EnterFunc(r)
	}
	return r.Name + ", ничего интересного. можно пройти - " + r.AvailablePaths()
}

// Player represents the player's state.
type Player struct {
	CurrentRoom *Room
	Inventory   []string
	HasBackpack bool
}

func (p *Player) HasItem(name string) bool {
	for _, item := range p.Inventory {
		if item == name {
			return true
		}
	}
	return false
}

// Game holds the entire game world state.
type Game struct {
	Player *Player
	Rooms  map[string]*Room
}

var game *Game

func initGame() {
	game = &Game{
		Rooms: make(map[string]*Room),
	}

	kitchen := &Room{
		Name: "кухня",
		Items: []*Item{
			{Name: "чай", Place: "на столе", Wearable: false},
		},
	}

	corridor := &Room{
		Name: "коридор",
	}

	room := &Room{
		Name: "комната",
		Items: []*Item{
			{Name: "ключи", Place: "на столе", Wearable: false},
			{Name: "конспекты", Place: "на столе", Wearable: false},
			{Name: "рюкзак", Place: "на стуле", Wearable: true},
		},
	}

	street := &Room{
		Name: "улица",
	}

	// Transitions
	kitchen.Transitions = []*Transition{
		{RoomName: "коридор", Room: corridor},
	}

	doorOpen := false
	door := &InteractiveObject{
		Name: "дверь",
		OnApply: func(item string) (string, bool) {
			if item == "ключи" {
				doorOpen = true
				return "дверь открыта", true
			}
			return "", false
		},
	}
	corridor.Objects = []*InteractiveObject{door}

	corridor.Transitions = []*Transition{
		{RoomName: "кухня", Room: kitchen},
		{RoomName: "комната", Room: room},
		{
			RoomName: "улица",
			Room:     street,
			CanPass: func() (bool, string) {
				if !doorOpen {
					return false, "дверь закрыта"
				}
				return true, ""
			},
		},
	}

	room.Transitions = []*Transition{
		{RoomName: "коридор", Room: corridor},
	}

	street.Transitions = []*Transition{
		{RoomName: "домой", Room: corridor},
	}

	// Look and Enter behaviors
	kitchen.LookFunc = func(r *Room, p *Player) string {
		tableDesc := ""
		if len(r.Items) > 0 {
			var names []string
			for _, it := range r.Items {
				names = append(names, it.Name)
			}
			tableDesc = "на столе: " + strings.Join(names, ", ") + ", "
		}
		task := "надо собрать рюкзак и идти в универ."
		if p.HasBackpack {
			task = "надо идти в универ."
		}
		return fmt.Sprintf("ты находишься на кухне, %s%s можно пройти - %s", tableDesc, task, r.AvailablePaths())
	}
	kitchen.EnterFunc = func(r *Room) string {
		return "кухня, ничего интересного. можно пройти - " + r.AvailablePaths()
	}

	corridor.LookFunc = func(r *Room, p *Player) string {
		return "ничего интересного. можно пройти - " + r.AvailablePaths()
	}
	corridor.EnterFunc = func(r *Room) string {
		return "ничего интересного. можно пройти - " + r.AvailablePaths()
	}

	room.LookFunc = func(r *Room, p *Player) string {
		if len(r.Items) == 0 {
			return "пустая комната. можно пройти - " + r.AvailablePaths()
		}
		var places []string
		placeItems := make(map[string][]string)
		for _, it := range r.Items {
			if _, exists := placeItems[it.Place]; !exists {
				places = append(places, it.Place)
			}
			placeItems[it.Place] = append(placeItems[it.Place], it.Name)
		}

		var parts []string
		for _, place := range places {
			parts = append(parts, place+": "+strings.Join(placeItems[place], ", "))
		}
		return strings.Join(parts, ", ") + ". можно пройти - " + r.AvailablePaths()
	}
	room.EnterFunc = func(r *Room) string {
		return "ты в своей комнате. можно пройти - " + r.AvailablePaths()
	}

	street.LookFunc = func(r *Room, p *Player) string {
		return "на улице весна. можно пройти - " + r.AvailablePaths()
	}
	street.EnterFunc = func(r *Room) string {
		return "на улице весна. можно пройти - " + r.AvailablePaths()
	}

	game.Rooms["кухня"] = kitchen
	game.Rooms["коридор"] = corridor
	game.Rooms["комната"] = room
	game.Rooms["улица"] = street

	game.Player = &Player{
		CurrentRoom: kitchen,
		Inventory:   []string{},
		HasBackpack: false,
	}
}

func handleCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "неизвестная команда"
	}

	action := parts[0]
	args := parts[1:]

	switch action {
	case "осмотреться":
		return handleLook()
	case "идти":
		return handleGo(args)
	case "надеть":
		return handleWear(args)
	case "взять":
		return handleTake(args)
	case "применить":
		return handleApply(args)
	default:
		return "неизвестная команда"
	}
}

func handleLook() string {
	return game.Player.CurrentRoom.Look(game.Player)
}

func handleGo(args []string) string {
	if len(args) < 1 {
		return "неизвестная команда"
	}
	destination := args[0]

	tr := game.Player.CurrentRoom.GetTransition(destination)
	if tr == nil {
		return "нет пути в " + destination
	}

	if tr.CanPass != nil {
		if ok, msg := tr.CanPass(); !ok {
			return msg
		}
	}

	game.Player.CurrentRoom = tr.Room
	return tr.Room.Enter()
}

func handleWear(args []string) string {
	if len(args) < 1 {
		return "неизвестная команда"
	}
	itemName := args[0]

	item := game.Player.CurrentRoom.FindItem(itemName)
	if item == nil || !item.Wearable {
		return "нет такого"
	}

	game.Player.CurrentRoom.RemoveItem(itemName)
	game.Player.HasBackpack = true
	return "вы надели: " + itemName
}

func handleTake(args []string) string {
	if len(args) < 1 {
		return "неизвестная команда"
	}
	itemName := args[0]

	if !game.Player.HasBackpack {
		return "некуда класть"
	}

	item := game.Player.CurrentRoom.FindItem(itemName)
	if item == nil || item.Wearable {
		return "нет такого"
	}

	game.Player.CurrentRoom.RemoveItem(itemName)
	game.Player.Inventory = append(game.Player.Inventory, itemName)
	return "предмет добавлен в инвентарь: " + itemName
}

func handleApply(args []string) string {
	if len(args) < 2 {
		return "неизвестная команда"
	}
	itemName := args[0]
	targetName := args[1]

	if !game.Player.HasItem(itemName) {
		return "нет предмета в инвентаре - " + itemName
	}

	obj := game.Player.CurrentRoom.GetObject(targetName)
	if obj == nil {
		return "не к чему применить"
	}

	if msg, ok := obj.OnApply(itemName); ok {
		return msg
	}

	return "не к чему применить"
}

func main() {
	initGame()
}
