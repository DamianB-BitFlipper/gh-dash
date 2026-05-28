package actionview

import (
	"charm.land/lipgloss/v2"
	data "github.com/dlvhdr/gh-dehub/v4/internal/data/actions"
)

func bucketToIcon(bucket data.CheckBucket, initialStyle lipgloss.Style, styles styles) string {
	switch bucket {
	case data.CheckBucketPass:
		return styles.successGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketFail:
		return styles.failureGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketNeutral:
		return styles.neutralGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketSkipping:
		return styles.skippedGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketCancel:
		return styles.canceledGlyph.Inherit(initialStyle).Render()
	default:
		return styles.pendingGlyph.Inherit(initialStyle).Render()
	}
}
