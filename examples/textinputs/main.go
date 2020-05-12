package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/boba"
	input "github.com/charmbracelet/boba/textinput"
	te "github.com/muesli/termenv"
)

var (
	color               = te.ColorProfile().Color
	focusedText         = "205"
	focusedPrompt       = te.String("> ").Foreground(color("205")).String()
	blurredPrompt       = "> "
	focusedSubmitButton = "[ " + te.String("Submit").Foreground(color("205")).String() + " ]"
	blurredSubmitButton = "[ " + te.String("Submit").Foreground(color("240")).String() + " ]"
)

func main() {
	if err := boba.NewProgram(
		initialize,
		update,
		view,
	).Start(); err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}
}

type Model struct {
	index         int
	nameInput     input.Model
	nickNameInput input.Model
	emailInput    input.Model
	submitButton  string
}

func initialize() (boba.Model, boba.Cmd) {
	name := input.NewModel()
	name.Placeholder = "Name"
	name.Focus()
	name.Prompt = focusedPrompt
	name.TextColor = focusedText

	nickName := input.NewModel()
	nickName.Placeholder = "Nickname"
	nickName.Prompt = blurredPrompt

	email := input.NewModel()
	email.Placeholder = "Email"
	email.Prompt = blurredPrompt

	return Model{0, name, nickName, email, blurredSubmitButton},
		boba.Batch(
			input.Blink(name),
			input.Blink(nickName),
			input.Blink(email),
		)

}

func update(msg boba.Msg, model boba.Model) (boba.Model, boba.Cmd) {
	m, ok := model.(Model)
	if !ok {
		panic("could not perform assertion on model")
	}

	var cmd boba.Cmd

	switch msg := msg.(type) {

	case boba.KeyMsg:
		switch msg.String() {

		case "ctrl+c":
			return m, boba.Quit

		// Cycle between inputs
		case "tab":
			fallthrough
		case "shift+tab":
			fallthrough
		case "enter":
			fallthrough
		case "up":
			fallthrough
		case "down":
			inputs := []input.Model{
				m.nameInput,
				m.nickNameInput,
				m.emailInput,
			}

			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, exit.
			if s == "enter" && m.index == len(inputs) {
				return m, boba.Quit
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				m.index--
			} else {
				m.index++
			}

			if m.index > len(inputs) {
				m.index = 0
			} else if m.index < 0 {
				m.index = len(inputs)
			}

			for i := 0; i <= len(inputs)-1; i++ {
				if i == m.index {
					// Focused input
					inputs[i].Focus()
					inputs[i].Prompt = focusedPrompt
					inputs[i].TextColor = focusedText
					continue
				}
				// Blurred input
				inputs[i].Blur()
				inputs[i].Prompt = blurredPrompt
				inputs[i].TextColor = ""
			}

			m.nameInput = inputs[0]
			m.nickNameInput = inputs[1]
			m.emailInput = inputs[2]

			if m.index == len(inputs) {
				m.submitButton = focusedSubmitButton
			} else {
				m.submitButton = blurredSubmitButton
			}

			return m, nil

		default:
			// Handle character input
			m, cmd = updateInputs(msg, m)
			return m, cmd
		}

	default:
		// Handle blinks
		m, cmd = updateInputs(msg, m)
		return m, cmd
	}
}

func updateInputs(msg boba.Msg, m Model) (Model, boba.Cmd) {
	var (
		cmd  boba.Cmd
		cmds []boba.Cmd
	)
	m.nameInput, cmd = input.Update(msg, m.nameInput)
	cmds = append(cmds, cmd)
	m.nickNameInput, cmd = input.Update(msg, m.nickNameInput)
	cmds = append(cmds, cmd)
	m.emailInput, cmd = input.Update(msg, m.emailInput)
	cmds = append(cmds, cmd)
	return m, boba.Batch(cmds...)
}

func view(model boba.Model) string {
	m, ok := model.(Model)
	if !ok {
		return "[error] could not perform assertion on model"
	}

	s := "\n"

	inputs := []string{
		input.View(m.nameInput),
		input.View(m.nickNameInput),
		input.View(m.emailInput),
	}

	for i := 0; i < len(inputs); i++ {
		s += inputs[i]
		if i < len(inputs)-1 {
			s += "\n"
		}
	}

	s += "\n\n" + m.submitButton + "\n"

	return s
}
