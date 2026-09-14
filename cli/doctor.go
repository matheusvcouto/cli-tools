package cli

const (
	builtinDoctorCommandID = "cli.builtin.doctor"
	builtinDoctorJSONID    = "cli.builtin.doctor.json"
)

func builtinDoctorCommand() Command {
	return Command{
		ID:      builtinDoctorCommandID,
		Name:    "doctor",
		Summary: "diagnose runtime capabilities and requirements",
		Flags: []Flag{{
			ID:      builtinDoctorJSONID,
			Long:    "json",
			Summary: "emit structured JSON",
			Action:  FlagSwitch,
		}},
	}
}

func (c *CompiledApp) installDoctorHandler() {
	c.handlers[builtinDoctorCommandID] = func(inv *Invocation) error {
		jsonMode, _ := ValueAs[bool](inv, builtinDoctorJSONID)
		return c.runDoctor(inv.Context, inv.IO, jsonMode)
	}
}
