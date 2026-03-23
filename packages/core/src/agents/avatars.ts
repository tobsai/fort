/**
 * Built-in avatar options for specialist agents.
 * Each is a small ASCII art piece that renders in the terminal.
 */

export interface Avatar {
  id: string;
  label: string;
  art: string;
}

export const BUILT_IN_AVATARS: Avatar[] = [
  {
    id: 'shield',
    label: 'Shield',
    art: [
      '  ╔═══╗  ',
      '  ║ ✦ ║  ',
      '  ║   ║  ',
      '  ╚╦═╦╝  ',
      '   ╚═╝   ',
    ].join('\n'),
  },
  {
    id: 'robot',
    label: 'Robot',
    art: [
      '  ┌───┐  ',
      '  │◉ ◉│  ',
      '  │ ═ │  ',
      '  └─┬─┘  ',
      '   ═╪═   ',
    ].join('\n'),
  },
  {
    id: 'owl',
    label: 'Owl',
    art: [
      '  /{◉◉}\\  ',
      '  ( ▼▼ )  ',
      '  /)  (\\  ',
      '  ""  ""  ',
    ].join('\n'),
  },
  {
    id: 'cat',
    label: 'Cat',
    art: [
      '  /\\_/\\  ',
      ' ( o.o ) ',
      '  > ^ <  ',
      '  /| |\\  ',
    ].join('\n'),
  },
  {
    id: 'compass',
    label: 'Compass',
    art: [
      '    N     ',
      '  ╔═╦═╗  ',
      ' W╠ ◆ ╣E ',
      '  ╚═╩═╝  ',
      '    S     ',
    ].join('\n'),
  },
  {
    id: 'lighthouse',
    label: 'Lighthouse',
    art: [
      '    ▲     ',
      '  ╔═╗    ',
      ' ═╣●╠═   ',
      '  ║ ║    ',
      ' ▓▓▓▓▓   ',
    ].join('\n'),
  },
  {
    id: 'brain',
    label: 'Brain',
    art: [
      '  ╭━━━╮  ',
      ' ╭┫ ◇ ┣╮ ',
      ' ╰┫   ┣╯ ',
      '  ╰━━━╯  ',
    ].join('\n'),
  },
  {
    id: 'wizard',
    label: 'Wizard',
    art: [
      '   /\\    ',
      '  /★ \\   ',
      ' /    \\  ',
      ' │◉  ◉│  ',
      ' │ ⌣  │  ',
    ].join('\n'),
  },
  {
    id: 'star',
    label: 'Star',
    art: [
      '    ★     ',
      '  ╱ | ╲   ',
      ' ━━━━━━━  ',
      '  ╲ | ╱   ',
      '    ★     ',
    ].join('\n'),
  },
  {
    id: 'fortress',
    label: 'Fortress',
    art: [
      ' ▄ ⚑ ▄   ',
      ' █████   ',
      ' █┌─┐█   ',
      ' █│ │█   ',
      ' ██████  ',
    ].join('\n'),
  },
];

export function getAvatarById(id: string): Avatar | undefined {
  return BUILT_IN_AVATARS.find((a) => a.id === id);
}
