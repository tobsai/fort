import React from 'react';
import ComponentCreator from '@docusaurus/ComponentCreator';

export default [
  {
    path: '/__docusaurus/debug',
    component: ComponentCreator('/__docusaurus/debug', '5ff'),
    exact: true
  },
  {
    path: '/__docusaurus/debug/config',
    component: ComponentCreator('/__docusaurus/debug/config', '5ba'),
    exact: true
  },
  {
    path: '/__docusaurus/debug/content',
    component: ComponentCreator('/__docusaurus/debug/content', 'a2b'),
    exact: true
  },
  {
    path: '/__docusaurus/debug/globalData',
    component: ComponentCreator('/__docusaurus/debug/globalData', 'c3c'),
    exact: true
  },
  {
    path: '/__docusaurus/debug/metadata',
    component: ComponentCreator('/__docusaurus/debug/metadata', '156'),
    exact: true
  },
  {
    path: '/__docusaurus/debug/registry',
    component: ComponentCreator('/__docusaurus/debug/registry', '88c'),
    exact: true
  },
  {
    path: '/__docusaurus/debug/routes',
    component: ComponentCreator('/__docusaurus/debug/routes', '000'),
    exact: true
  },
  {
    path: '/',
    component: ComponentCreator('/', '563'),
    routes: [
      {
        path: '/',
        component: ComponentCreator('/', 'd70'),
        routes: [
          {
            path: '/',
            component: ComponentCreator('/', '4e6'),
            routes: [
              {
                path: '/architecture/ipc',
                component: ComponentCreator('/architecture/ipc', '393'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/architecture/module-bus',
                component: ComponentCreator('/architecture/module-bus', '8b0'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/architecture/overview',
                component: ComponentCreator('/architecture/overview', 'f3c'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/architecture/task-graph',
                component: ComponentCreator('/architecture/task-graph', 'f14'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli',
                component: ComponentCreator('/cli', '053'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/agents',
                component: ComponentCreator('/cli/agents', 'de8'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/behaviors',
                component: ComponentCreator('/cli/behaviors', 'af6'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/doctor',
                component: ComponentCreator('/cli/doctor', 'afe'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/flags',
                component: ComponentCreator('/cli/flags', 'dce'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/harness',
                component: ComponentCreator('/cli/harness', 'e83'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/introspect',
                component: ComponentCreator('/cli/introspect', '33a'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/llm',
                component: ComponentCreator('/cli/llm', '8f3'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/memory',
                component: ComponentCreator('/cli/memory', 'db8'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/plugins',
                component: ComponentCreator('/cli/plugins', 'def'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/rewind',
                component: ComponentCreator('/cli/rewind', 'aaa'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/routines',
                component: ComponentCreator('/cli/routines', '80d'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/schedule',
                component: ComponentCreator('/cli/schedule', '0ee'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/status',
                component: ComponentCreator('/cli/status', '0be'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/tasks',
                component: ComponentCreator('/cli/tasks', '85d'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/threads',
                component: ComponentCreator('/cli/threads', 'c8c'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/tokens',
                component: ComponentCreator('/cli/tokens', '090'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/cli/tools',
                component: ComponentCreator('/cli/tools', '18e'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/getting-started/authentication',
                component: ComponentCreator('/getting-started/authentication', 'fd3'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/getting-started/first-steps',
                component: ComponentCreator('/getting-started/first-steps', '7cf'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/getting-started/installation',
                component: ComponentCreator('/getting-started/installation', 'e0c'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/agents',
                component: ComponentCreator('/guides/agents', '816'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/behaviors',
                component: ComponentCreator('/guides/behaviors', '684'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/feature-flags',
                component: ComponentCreator('/guides/feature-flags', 'b72'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/flows',
                component: ComponentCreator('/guides/flows', 'cf0'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/integrations',
                component: ComponentCreator('/guides/integrations', 'ac7'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/introspection',
                component: ComponentCreator('/guides/introspection', '66e'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/llm',
                component: ComponentCreator('/guides/llm', '158'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/macos-native',
                component: ComponentCreator('/guides/macos-native', 'f7f'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/memory',
                component: ComponentCreator('/guides/memory', 'a6b'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/permissions',
                component: ComponentCreator('/guides/permissions', '755'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/plugins',
                component: ComponentCreator('/guides/plugins', '704'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/rewind',
                component: ComponentCreator('/guides/rewind', '244'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/routines',
                component: ComponentCreator('/guides/routines', '9bc'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/self-coding',
                component: ComponentCreator('/guides/self-coding', '990'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/threads',
                component: ComponentCreator('/guides/threads', 'c8e'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/tokens-and-budgets',
                component: ComponentCreator('/guides/tokens-and-budgets', 'cf3'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/guides/tools',
                component: ComponentCreator('/guides/tools', 'd81'),
                exact: true,
                sidebar: "docs"
              },
              {
                path: '/',
                component: ComponentCreator('/', '7da'),
                exact: true,
                sidebar: "docs"
              }
            ]
          }
        ]
      }
    ]
  },
  {
    path: '*',
    component: ComponentCreator('*'),
  },
];
