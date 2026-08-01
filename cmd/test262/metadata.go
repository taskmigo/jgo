package main

import (
	"errors"
	"strings"
)

func frontmatter(source string) (metadata, string, error) {
	start := strings.Index(source, "/*---")
	if start < 0 {
		return metadata{}, "", errors.New("malformed frontmatter: opening marker missing")
	}
	endOffset := strings.Index(source[start+5:], "---*/")
	if endOffset < 0 {
		return metadata{}, "", errors.New("malformed frontmatter: closing marker missing")
	}
	end := start + 5 + endOffset
	header := source[start+5 : end]
	parsed := parseMetadata(strings.Split(header, "\n"))
	return parsed, source[end+5:], nil
}

func parseMetadata(lines []string) metadata {
	parsed := metadata{}
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "features":
			parsed.features = parseList(value, lines, &index)
		case "flags":
			parsed.flags = parseList(value, lines, &index)
		case "includes":
			parsed.includes = parseList(value, lines, &index)
		case "negative":
			parsed.negative = true
		case "phase":
			if parsed.negative && unquoteMetadataValue(value) == "parse" {
				parsed.negativeParse = true
			}
		case "type":
			if parsed.negative {
				parsed.negativeType = unquoteMetadataValue(value)
			}
		}
	}
	return parsed
}

func parseList(value string, lines []string, index *int) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") {
		value = strings.Trim(value, "[] ")
		if value == "" {
			return nil
		}
		items := strings.Split(value, ",")
		for itemIndex := range items {
			items[itemIndex] = unquoteMetadataValue(strings.TrimSpace(items[itemIndex]))
		}
		return items
	}

	var items []string
	for *index+1 < len(lines) {
		next := strings.TrimSpace(lines[*index+1])
		if !strings.HasPrefix(next, "-") {
			break
		}
		*index++
		items = append(items, unquoteMetadataValue(strings.TrimSpace(strings.TrimPrefix(next, "-"))))
	}
	return items
}

func unquoteMetadataValue(value string) string {
	return strings.Trim(value, " '\"")
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
