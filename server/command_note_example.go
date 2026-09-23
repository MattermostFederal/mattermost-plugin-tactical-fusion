package main

import (
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/note"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

type noteExample struct {
	heading string
	text    string
	notes   []noteEntry
}

type noteEntry struct {
	label    string
	markdown string
}

const noteExamplesFooter = "\n\nHover a link to read its note, or click it to open the note in the sidebar. " +
	"Write your own with `/" + commandTrigger + " note <label> | <markdown>`."

var noteExamples = []noteExample{
	{
		heading: "Markdown notes: a glossary",
		text: "Today's {ATO} tasks {DCA} and {OCA} sorties inside the {ACO}. {ISR} feeds the {COP}, " +
			"{CAS} and {TST} strikes follow the {ROE}, and {BDA} closes the loop.",
		notes: []noteEntry{
			{"ATO", "**ATO**: Air Tasking Order\n\nThe order that assigns every sortie, target and mission for one air tasking day."},
			{"DCA", "**DCA**: Defensive Counter Air\n\nProtects friendly forces and assets from enemy air and missile attack.\n\n- *Active:* fighters and surface-to-air missiles\n- *Passive:* camouflage, dispersal and hardening"},
			{"OCA", "**OCA**: Offensive Counter Air\n\nDestroys or neutralizes enemy aircraft, missiles and their launch and support systems, as close to their source as possible."},
			{"ACO", "**ACO**: Airspace Control Order\n\nPublishes the airspace control measures (corridors, restricted areas, altitudes) that deconflict everything flying on the ATO."},
			{"ISR", "**ISR**: Intelligence, Surveillance and Reconnaissance\n\nCollects and processes information on the adversary and the operating environment."},
			{"COP", "**COP**: Common Operational Picture\n\nOne shared display of relevant information, so every echelon works from the same view."},
			{"CAS", "**CAS**: Close Air Support\n\nAir action against hostile targets in close proximity to friendly ground forces, integrated with their fire and movement."},
			{"TST", "**TST**: Time-Sensitive Target\n\nA target of such high priority, or so fleeting, that it needs an immediate response."},
			{"ROE", "**ROE**: Rules of Engagement\n\nThe directives that set when, where and how force may be used."},
			{"BDA", "**BDA**: Battle Damage Assessment\n\nEstimates the damage a strike achieved, to decide whether to strike again."},
		},
	},
	{
		heading: "Markdown notes: tables",
		text:    "Morning stand-up: {squadron readiness} is green with one tail down, and the {range schedule} is set for today.",
		notes: []noteEntry{
			{"squadron readiness", "#### Squadron readiness\n\n" +
				"| Tail | Status | Fuel | Next inspection |\n" +
				"|:--|:--|--:|:--|\n" +
				"| 101 | :white_check_mark: FMC | 12,400 lb | 26 Sep |\n" +
				"| 102 | :white_check_mark: FMC | 11,950 lb | 30 Sep |\n" +
				"| 103 | :warning: PMC | 9,800 lb | 24 Sep |\n" +
				"| 104 | :x: NMC | n/a | Engine change |\n\n" +
				"**3 of 4** mission capable."},
			{"range schedule", "#### Range schedule\n\n" +
				"| Window | Range | Flight | Notes |\n" +
				"|:--|:--|:--|:--|\n" +
				"| 1400-1530Z | Range 7 | Alpha | Live fire |\n" +
				"| 1600-1730Z | Range 9 | Bravo | Dry passes only |\n" +
				"| 1800-1900Z | Range 7 | Charlie | Night, NVG |\n\n" +
				"> Range 9 closes early if the wind exceeds **25 kt**."},
		},
	},
	{
		heading: "Markdown notes: lists, quotes and code",
		text:    "Before you step, read the {weather brief} and run the {launch checklist}.",
		notes: []noteEntry{
			{"weather brief", "#### Weather brief\n\n" +
				"**Valid** 1800Z to 0200Z\n\n" +
				"- Wind 070 at 12 kt, gusting 20\n" +
				"- Scattered at 2,500 ft, broken at 6,000 ft\n" +
				"- Isolated showers after 2200Z\n\n" +
				"| Field | Ceiling | Visibility |\n" +
				"|:--|--:|--:|\n" +
				"| Home | 6,000 ft | 10 SM |\n" +
				"| Alternate | 4,500 ft | 7 SM |\n\n" +
				"> Expect moderate turbulence below 3,000 ft on the windward side."},
			{"launch checklist", "#### Launch checklist\n\n" +
				"- [x] Flight plan filed\n" +
				"- [x] NOTAMs reviewed\n" +
				"- [ ] Weapons safe check\n" +
				"- [ ] Check in on `251.1 MHz`\n\n" +
				"1. Taxi at **1745Z**\n" +
				"2. Takeoff at **1800Z**\n" +
				"3. ~~Hot pit~~ canceled, refuel on the ground"},
		},
	},
}

func noteExampleMessage(tagger *decorators.Tagger, example noteExample) string {
	pairs := make([]string, 0, 2*len(example.notes))
	for _, entry := range example.notes {
		pairs = append(pairs, "{"+entry.label+"}", noteLink(tagger, entry.label, entry.markdown))
	}
	return "#### " + example.heading + "\n\n" + strings.NewReplacer(pairs...).Replace(example.text) + noteExamplesFooter
}

func noteLink(tagger *decorators.Tagger, label, markdown string) string {
	params, ok := (&note.Decorator{}).Parse(markdown, time.Time{})
	if !ok {
		return label
	}
	return tagger.LinkFor(note.Type, label, params)
}

func (p *Plugin) noteExampleMessages() []string {
	tagger := &decorators.Tagger{URLPrefix: p.decorateURLPrefix()}
	messages := make([]string, 0, len(noteExamples))
	for _, example := range noteExamples {
		messages = append(messages, noteExampleMessage(tagger, example))
	}
	return messages
}

func (p *Plugin) postNoteExamples(args *model.CommandArgs) int {
	failed := 0
	for _, message := range p.noteExampleMessages() {
		if _, appErr := p.API.CreatePost(examplePost(args, message)); appErr != nil {
			failed++
			p.API.LogError("tactical-fusion: could not post a note example",
				"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		}
	}
	return failed
}
