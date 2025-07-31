package scribe_test

import (
	"math"
	"testing"

	scribe "github.com/kofi-q/scribe-go"
	"github.com/stretchr/testify/require"
)

func TestScratchInit(t *testing.T) {
	doc := scribe.New("P", "pt", "letter", "")

	doc.SetFont("Helvetica", scribe.FontStyleNone, 12)

	scratch, err := doc.Scratch(42)
	require.NoError(t, err)

	require.Equal(t, 0.0, scratch.X())
	require.Equal(t, 0.0, scratch.Y())

	x, y := scratch.Xy()
	require.Equal(t, 0.0, x)
	require.Equal(t, 0.0, y)
}

func TestScratchSimpleText(t *testing.T) {
	doc := scribe.New("P", "pt", "letter", "")

	doc.SetFont("Helvetica", scribe.FontStyleNone, 12)
	scratch, err := doc.Scratch(doc.GetStringWidth("The quick, brown fox"))
	require.NoError(t, err)

	scratch.Text(15, "The quick, ")
	require.Equal(t, doc.GetStringWidth("The quick, "), scratch.X())
	require.Equal(t, 0.0, scratch.Y())

	scratch.Text(15, "brown fox")
	require.Equal(t, doc.GetStringWidth("The quick, brown fox"), scratch.X())
	require.Equal(t, 0.0, scratch.Y())

	scratch.Text(15, ".")
	require.Equal(t, doc.GetStringWidth("fox."), scratch.X())
	require.Equal(t, 15.0, scratch.Y())

	scratch.Text(15, "\n")
	require.Equal(t, 0.0, scratch.X())
	require.Equal(t, 30.0, scratch.Y())

	scratch.Text(15, "\n")
	require.Equal(t, 0.0, scratch.X())
	require.Equal(t, 45.0, scratch.Y())

	scratch.Text(15, "It jumped.")
	require.Equal(t, doc.GetStringWidth("It jumped."), scratch.X())
	require.Equal(t, 45.0, scratch.Y())

	scratch.Ln(30)
	require.Equal(t, 0.0, scratch.X())
	require.Equal(t, 75.0, scratch.Y())

	scratch.Text(15, "The quick, brown fox jumped over a dog.")
	require.Equal(t, doc.GetStringWidth("jumped over a dog."), scratch.X())
	require.Equal(t, 90.0, scratch.Y())
}

func TestScratchRichText(t *testing.T) {
	epsilon := math.Nextafter(1.0, 2.0) - 1.0

	doc := scribe.New("P", "pt", "letter", "")

	doc.SetFont("Helvetica", scribe.FontStyleNone, 12)
	scratch, err := doc.Scratch(doc.GetStringWidth("The quick, brown fox"))
	require.NoError(t, err)

	scratch.Text(15, "The quick, ")
	require.Equal(t, doc.GetStringWidth("The quick, "), scratch.X())
	require.Equal(t, 0.0, scratch.Y())

	scratch.SetFont("Helvetica", scribe.FontStyleB, 12)
	doc.SetFont("Helvetica", scribe.FontStyleNone, 12) // No-op for scratch pad.
	scratch.Text(15, "brown")
	{
		width1 := doc.GetStringWidth("The quick, ")

		doc.SetFont("Helvetica", scribe.FontStyleB, 12)
		width2 := doc.GetStringWidth("brown")

		require.InEpsilon(t, width1+width2, scratch.X(), epsilon)

		doc.SetFont("Helvetica", scribe.FontStyleNone, 12)
	}
	require.Equal(t, 0.0, scratch.Y())

	scratch.SetFont("Helvetica", scribe.FontStyleNone, 12)
	scratch.Text(15, " fox")
	require.Equal(t, doc.GetStringWidth("fox"), scratch.X())
	require.Equal(t, 15.0, scratch.Y())
}
