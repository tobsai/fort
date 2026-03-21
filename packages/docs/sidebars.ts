import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/installation',
        'getting-started/authentication',
        'getting-started/first-steps',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/agents',
        'guides/llm',
        'guides/memory',
        'guides/threads',
        'guides/behaviors',
        'guides/routines',
        'guides/flows',
        'guides/tokens-and-budgets',
        'guides/tools',
        'guides/permissions',
        'guides/feature-flags',
        'guides/plugins',
        'guides/self-coding',
        'guides/rewind',
        'guides/introspection',
        'guides/integrations',
        'guides/macos-native',
      ],
    },
    {
      type: 'category',
      label: 'CLI Reference',
      link: { type: 'doc', id: 'cli/index' },
      items: [
        'cli/doctor',
        'cli/status',
        'cli/llm',
        'cli/agents',
        'cli/tasks',
        'cli/threads',
        'cli/memory',
        'cli/tools',
        'cli/tokens',
        'cli/behaviors',
        'cli/routines',
        'cli/schedule',
        'cli/flags',
        'cli/plugins',
        'cli/harness',
        'cli/rewind',
        'cli/introspect',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/overview',
        'architecture/module-bus',
        'architecture/task-graph',
        'architecture/ipc',
      ],
    },
  ],
};

export default sidebars;
