package petstore

import "context"

func (e *ExampleFramework) ProcessOrderExample(ctx context.Context) error {
	e.Story("This example demonstrates processing a sales order")
	e.Tests("Validates order processing workflow and payment handling")
	return nil
}

func (e *ExampleFramework) GenerateInvoiceExample(ctx context.Context) error {
	e.Story("This example shows how to generate customer invoices")
	e.Tests("Validates invoice generation and delivery")
	return nil
}
