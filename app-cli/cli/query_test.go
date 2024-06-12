package cli

import "testing"

func TestKeyword_NonExistingKeyword(t *testing.T) {
	// Create a new context with an empty grammar
	c := &Context{
		Grammar: []Option{},
	}

	// Call the Keyword function with a non-existing keyword
	index, found := c.Keyword("nonexistent")

	// Assert that the function returns the expected index and found status
	if index != -1 || found {
		t.Errorf("Expected index -1 and found status false, but got index %d and found status true", index)
	}
}

func TestKeyword_NonExistingKeywordInSubcommand(t *testing.T) {
	// Create a new context with a grammar containing a subcommand
	subContext := &Context{
		Grammar: []Option{},
	}
	c := &Context{
		Grammar: []Option{
			{
				OptionType: Subcommand,
				Found:      true,
				Value:      subContext,
			},
		},
	}

	// Call the Keyword function with a non-existing keyword in the subcommand
	index, found := c.Keyword("nonexistent")

	// Assert that the function returns the expected index and found status
	if index != -1 || found {
		t.Errorf("Expected index -1 and found status false, but got index %d and found status true", index)
	}
}

func TestKeyword_ExistingKeyword(t *testing.T) {
	// Create a new context with a grammar containing a keyword
	c := &Context{
		Grammar: []Option{
			{
				OptionType: KeywordType,
				Found:      true,
				LongName:   "existing",
				Keywords:   []string{"existing"},
				Value:      "existing",
			},
		},
	}

	// Call the Keyword function with an existing keyword
	index, found := c.Keyword("existing")

	// Assert that the function returns the expected index and found status
	if index != 0 || !found {
		t.Errorf("Expected index 0 and found status true, but got index %d and found status false", index)
	}
}

func TestKeyword_ExistingKeywordInSubcommand(t *testing.T) {
	// Create a new context with a grammar containing a subcommand with a keyword
	subContext := &Context{
		Grammar: []Option{
			{
				OptionType: KeywordType,
				Found:      true,
				LongName:   "existing",
				Keywords:   []string{"existing"},
				Value:      "existing",
			},
		},
	}
	c := &Context{
		Grammar: []Option{
			{
				OptionType: Subcommand,
				Found:      true,
				Value:      subContext,
			},
		},
	}

	// Call the Keyword function with an existing keyword in the subcommand
	index, found := c.Keyword("existing")

	// Assert that the function returns the expected index and found status
	if index != 0 || !found {
		t.Errorf("Expected index 0 and found status true, but got index %d and found status false", index)
	}
}

func TestKeyword_CaseInsensitiveKeyword(t *testing.T) {
	// Create a new context with a grammar containing a keyword
	c := &Context{
		Grammar: []Option{
			{
				OptionType: KeywordType,
				Found:      true,
				LongName:   "existing",
				Keywords:   []string{"EXISTING"},
				Value:      "existing",
			},
		},
	}

	// Call the Keyword function with a keyword in a different case
	index, found := c.Keyword("Existing")

	// Assert that the function returns the expected index and found status
	if index != 0 || !found {
		t.Errorf("Expected index 0 and found status true, but got index %d and found status false", index)
	}
}
