package prrow

import (
	"time"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
)

type Data struct {
	Primary    *data.PullRequestData
	Enriched   data.EnrichedPullRequestData
	IsEnriched bool
}

func (data Data) GetTitle() string {
	return data.Primary.Title
}

func (data Data) GetRepoNameWithOwner() string {
	return data.Primary.Repository.NameWithOwner
}

func (data Data) GetNumber() int {
	return data.Primary.Number
}

func (data Data) GetIsDraft() bool {
	return data.Primary.IsDraft
}

func (data Data) GetUrl() string {
	return data.Primary.Url
}

func (data Data) GetUpdatedAt() time.Time {
	return data.Primary.UpdatedAt
}

func (data Data) GetCreatedAt() time.Time {
	return data.Primary.CreatedAt
}
