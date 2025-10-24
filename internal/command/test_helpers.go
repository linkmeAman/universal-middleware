package command

// SetValidator sets a custom validator for testing
func (p *Processor) SetValidator(v CommandValidator) {
	p.validator = v
}
