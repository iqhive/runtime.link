package petstore

import "context"

func (e *ExampleFramework) CreateCampaignExample(ctx context.Context) error {
	e.Story("This example demonstrates creating a marketing campaign")
	e.Tests("Validates campaign creation with proper targeting")
	return nil
}

func (e *ExampleFramework) TrackConversionExample(ctx context.Context) error {
	e.Story("This example shows how to track marketing conversions")
	e.Tests("Validates conversion tracking and attribution")
	return nil
}
