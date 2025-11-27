package util

import "strings"

func PluralizeWord(word string) string {
	// Check if the word is already in plural form
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "es") {
		return word // Return the word unchanged if it's plural
	}

	endings := []string{"s", "sh", "ch", "x", "z"}

	for _, ending := range endings {
		if strings.HasSuffix(word, ending) {
			return word + "es" // Add "es" to make it plural
		}
	}
	if len(word) > 1 && strings.HasSuffix(word, "y") && !isVowel(word[len(word)-2]) {
        return word[:len(word)-1] + "ies"
    }

	return word + "s" // Default to adding "s" if no special ending is found
}

func isVowel(char byte) bool {
    vowels := "aeiouAEIOU"
    return strings.ContainsRune(vowels, rune(char))
}