// Package sentiment provides keyword-based sentiment analysis.
package sentiment

import (
	"strings"
	"unicode"
)

// Sentiment represents the sentiment type.
type Sentiment string

const (
	Bullish Sentiment = "BULLISH"
	Bearish Sentiment = "BEARISH"
	Neutral Sentiment = "NEUTRAL"
)

// Result represents the sentiment analysis result.
type Result struct {
	Sentiment       Sentiment `json:"sentiment"`
	Score           float64   `json:"score"` // -1 to 1
	BullishKeywords []string  `json:"bullish_keywords"`
	BearishKeywords []string  `json:"bearish_keywords"`
	Confidence      float64   `json:"confidence"` // 0 to 1
}

// Analyzer provides keyword-based sentiment analysis.
type Analyzer struct {
	bullishWords map[string]float64
	bearishWords map[string]float64
	intensifiers map[string]float64
	negators     map[string]bool
}

// NewAnalyzer creates a new sentiment analyzer with predefined dictionaries.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		bullishWords: getBullishDictionary(),
		bearishWords: getBearishDictionary(),
		intensifiers: getIntensifiers(),
		negators:     getNegators(),
	}
}

// Analyze performs sentiment analysis on the given text.
func (a *Analyzer) Analyze(text string) *Result {
	words := tokenize(text)

	var bullishScore, bearishScore float64
	var bullishKeywords, bearishKeywords []string
	var totalWords int

	for i, word := range words {
		word = strings.ToLower(word)

		// Check for negation in previous words
		negated := a.isNegated(words, i)

		// Check for intensifier
		intensity := a.getIntensity(words, i)

		// Check bullish words
		if score, ok := a.bullishWords[word]; ok {
			if negated {
				bearishScore += score * intensity
				bearishKeywords = append(bearishKeywords, word)
			} else {
				bullishScore += score * intensity
				bullishKeywords = append(bullishKeywords, word)
			}
			totalWords++
		}

		// Check bearish words
		if score, ok := a.bearishWords[word]; ok {
			if negated {
				bullishScore += score * intensity
				bullishKeywords = append(bullishKeywords, word)
			} else {
				bearishScore += score * intensity
				bearishKeywords = append(bearishKeywords, word)
			}
			totalWords++
		}
	}

	// Calculate final score (-1 to 1)
	var score float64
	totalScore := bullishScore + bearishScore
	if totalScore > 0 {
		score = (bullishScore - bearishScore) / totalScore
	}

	// Determine sentiment
	var sentiment Sentiment
	switch {
	case score > 0.1:
		sentiment = Bullish
	case score < -0.1:
		sentiment = Bearish
	default:
		sentiment = Neutral
	}

	// Calculate confidence based on number of sentiment words found
	confidence := float64(totalWords) / float64(len(words)+1)
	if confidence > 1 {
		confidence = 1
	}

	return &Result{
		Sentiment:       sentiment,
		Score:           score,
		BullishKeywords: unique(bullishKeywords),
		BearishKeywords: unique(bearishKeywords),
		Confidence:      confidence,
	}
}

// isNegated checks if the word at position i is negated.
func (a *Analyzer) isNegated(words []string, i int) bool {
	// Check previous 3 words for negation
	start := i - 3
	if start < 0 {
		start = 0
	}

	for j := start; j < i; j++ {
		if a.negators[strings.ToLower(words[j])] {
			return true
		}
	}
	return false
}

// getIntensity returns the intensity multiplier for the word at position i.
func (a *Analyzer) getIntensity(words []string, i int) float64 {
	// Check previous 2 words for intensifiers
	start := i - 2
	if start < 0 {
		start = 0
	}

	for j := start; j < i; j++ {
		if mult, ok := a.intensifiers[strings.ToLower(words[j])]; ok {
			return mult
		}
	}
	return 1.0
}

// tokenize splits text into words.
func tokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current.WriteRune(unicode.ToLower(r))
		} else if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// unique returns unique strings from a slice.
func unique(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// getBullishDictionary returns the bullish keywords dictionary.
func getBullishDictionary() map[string]float64 {
	return map[string]float64{
		// Strong bullish
		"surge":        1.0,
		"surges":       1.0,
		"surging":      1.0,
		"soar":         1.0,
		"soars":        1.0,
		"soaring":      1.0,
		"rally":        0.9,
		"rallies":      0.9,
		"rallying":     0.9,
		"boom":         0.9,
		"booming":      0.9,
		"skyrocket":    1.0,
		"skyrockets":   1.0,
		"breakthrough": 0.9,
		"breakout":     0.8,

		// Positive performance
		"gain":        0.7,
		"gains":       0.7,
		"gaining":     0.7,
		"rise":        0.6,
		"rises":       0.6,
		"rising":      0.6,
		"up":          0.4,
		"higher":      0.5,
		"high":        0.4,
		"increase":    0.6,
		"increases":   0.6,
		"increasing":  0.6,
		"growth":      0.7,
		"growing":     0.7,
		"grow":        0.6,
		"advance":     0.6,
		"advances":    0.6,
		"advancing":   0.6,
		"climb":       0.6,
		"climbs":      0.6,
		"climbing":    0.6,
		"jump":        0.7,
		"jumps":       0.7,
		"jumping":     0.7,
		"recover":     0.6,
		"recovers":    0.6,
		"recovery":    0.7,
		"rebound":     0.6,
		"rebounds":    0.6,
		"rebounding":  0.6,

		// Positive outlook
		"bullish":     0.9,
		"optimistic":  0.8,
		"optimism":    0.8,
		"positive":    0.6,
		"upbeat":      0.7,
		"confident":   0.6,
		"confidence":  0.6,
		"strong":      0.6,
		"strength":    0.6,
		"robust":      0.7,
		"solid":       0.5,
		"healthy":     0.5,
		"outperform":  0.8,
		"outperforms": 0.8,
		"beat":        0.6,
		"beats":       0.6,
		"beating":     0.6,
		"exceed":      0.7,
		"exceeds":     0.7,
		"exceeding":   0.7,
		"upgrade":     0.8,
		"upgrades":    0.8,
		"upgraded":    0.8,
		"buy":         0.7,
		"accumulate":  0.7,

		// Positive fundamentals
		"profit":      0.7,
		"profits":     0.7,
		"profitable":  0.7,
		"earnings":    0.5,
		"revenue":     0.4,
		"dividend":    0.5,
		"expansion":   0.6,
		"expand":      0.6,
		"expands":     0.6,
		"expanding":   0.6,

		// Indian market specific
		"nifty":       0.3,
		"sensex":      0.3,
		"fii":         0.3,
		"dii":         0.3,
		"multibagger": 0.9,
	}
}

// getBearishDictionary returns the bearish keywords dictionary.
func getBearishDictionary() map[string]float64 {
	return map[string]float64{
		// Strong bearish
		"crash":      1.0,
		"crashes":    1.0,
		"crashing":   1.0,
		"plunge":     1.0,
		"plunges":    1.0,
		"plunging":   1.0,
		"collapse":   1.0,
		"collapses":  1.0,
		"collapsing": 1.0,
		"tank":       0.9,
		"tanks":      0.9,
		"tanking":    0.9,
		"tumble":     0.9,
		"tumbles":    0.9,
		"tumbling":   0.9,
		"freefall":   1.0,

		// Negative performance
		"fall":        0.7,
		"falls":       0.7,
		"falling":     0.7,
		"drop":        0.7,
		"drops":       0.7,
		"dropping":    0.7,
		"decline":     0.7,
		"declines":    0.7,
		"declining":   0.7,
		"down":        0.4,
		"lower":       0.5,
		"low":         0.4,
		"decrease":    0.6,
		"decreases":   0.6,
		"decreasing":  0.6,
		"loss":        0.8,
		"losses":      0.8,
		"losing":      0.7,
		"lose":        0.7,
		"slip":        0.5,
		"slips":       0.5,
		"slipping":    0.5,
		"slide":       0.6,
		"slides":      0.6,
		"sliding":     0.6,
		"sink":        0.7,
		"sinks":       0.7,
		"sinking":     0.7,
		"dip":         0.5,
		"dips":        0.5,
		"dipping":     0.5,
		"retreat":     0.5,
		"retreats":    0.5,
		"retreating":  0.5,

		// Negative outlook
		"bearish":       0.9,
		"pessimistic":   0.8,
		"pessimism":     0.8,
		"negative":      0.6,
		"weak":          0.6,
		"weakness":      0.6,
		"concern":       0.5,
		"concerns":      0.5,
		"worried":       0.6,
		"worry":         0.6,
		"worries":       0.6,
		"fear":          0.7,
		"fears":         0.7,
		"uncertain":     0.5,
		"uncertainty":   0.6,
		"volatile":      0.5,
		"volatility":    0.5,
		"risk":          0.4,
		"risks":         0.4,
		"risky":         0.5,
		"underperform":  0.8,
		"underperforms": 0.8,
		"miss":          0.6,
		"misses":        0.6,
		"missing":       0.6,
		"downgrade":     0.8,
		"downgrades":    0.8,
		"downgraded":    0.8,
		"sell":          0.7,
		"selling":       0.6,

		// Negative fundamentals
		"debt":       0.5,
		"default":    0.9,
		"defaults":   0.9,
		"bankrupt":   1.0,
		"bankruptcy": 1.0,
		"fraud":      1.0,
		"scam":       1.0,
		"scandal":    0.9,

		// Market conditions
		"correction":  0.6,
		"selloff":     0.8,
		"bloodbath":   1.0,
		"carnage":     1.0,
		"meltdown":    1.0,
		"recession":   0.8,
		"inflation":   0.5,
		"crisis":      0.9,
	}
}

// getIntensifiers returns words that intensify sentiment.
func getIntensifiers() map[string]float64 {
	return map[string]float64{
		"very":        1.5,
		"extremely":   2.0,
		"highly":      1.5,
		"significantly": 1.5,
		"substantially": 1.5,
		"massively":   2.0,
		"hugely":      1.8,
		"sharply":     1.5,
		"strongly":    1.3,
		"dramatically": 1.8,
		"tremendously": 1.8,
		"absolutely":  1.5,
		"completely":  1.5,
		"totally":     1.5,
		"major":       1.3,
		"huge":        1.5,
		"big":         1.2,
		"massive":     1.8,
	}
}

// getNegators returns words that negate sentiment.
func getNegators() map[string]bool {
	return map[string]bool{
		"not":     true,
		"no":      true,
		"never":   true,
		"neither": true,
		"nobody":  true,
		"nothing": true,
		"nowhere": true,
		"none":    true,
		"hardly":  true,
		"barely":  true,
		"scarcely": true,
		"without": true,
		"dont":    true,
		"doesn":   true,
		"didnt":   true,
		"wont":    true,
		"wouldnt": true,
		"couldnt": true,
		"shouldnt": true,
		"cant":    true,
		"cannot":  true,
		"isnt":    true,
		"arent":   true,
		"wasnt":   true,
		"werent":  true,
		"hasnt":   true,
		"havent":  true,
		"hadnt":   true,
		"unlikely": true,
		"fail":    true,
		"fails":   true,
		"failed":  true,
	}
}

