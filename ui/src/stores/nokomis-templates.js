// Nokomis Structure Templates
// These templates define common organizational documentation structures

export const structureTemplates = {
  // Living / Operational Document Set
  // Active documents in use, requiring tracking and revision control
  livingOperational: {
    id: 'living-operational',
    name: 'Living / Operational Document Set',
    description: 'Active documentation in regular use, requiring tracking and revision control for reliability',
    category: 'operational',
    structure: [
      {
        name: 'Operations',
        type: 'area',
        description: 'Operational procedures and runbooks',
        children: [
          { name: 'Runbooks', type: 'feature', description: 'Step-by-step operational procedures' },
          { name: 'Incident Response', type: 'feature', description: 'Incident handling procedures' },
          { name: 'Maintenance Procedures', type: 'feature', description: 'Regular maintenance tasks' },
        ],
      },
      {
        name: 'Standards',
        type: 'area',
        description: 'Operational standards and guidelines',
        children: [
          { name: 'Security Standards', type: 'feature', description: 'Security policies and procedures' },
          { name: 'Compliance', type: 'feature', description: 'Compliance documentation' },
          { name: 'Best Practices', type: 'feature', description: 'Recommended practices' },
        ],
      },
      {
        name: 'Reference',
        type: 'area',
        description: 'Reference documentation',
        children: [
          { name: 'Architecture', type: 'feature', description: 'System architecture documentation' },
          { name: 'APIs', type: 'feature', description: 'API documentation' },
          { name: 'Configuration', type: 'feature', description: 'Configuration guides' },
        ],
      },
    ],
    metadata: {
      revisionControl: true,
      lifecycleTracking: true,
      approvalRequired: true,
      updateFrequency: 'continuous',
    },
  },

  // Delivery Set of Documentation
  // Project delivery docs with known titles, relations, and RACI definitions
  deliverySet: {
    id: 'delivery-set',
    name: 'Delivery Set of Documentation',
    description: 'Documentation set required for project delivery, with defined titles, relations, and RACI responsibilities',
    category: 'delivery',
    structure: [
      {
        name: 'Project Initiation',
        type: 'project',
        description: 'Project kickoff documentation',
        children: [
          { name: 'Project Charter', type: 'feature', description: 'Project scope and objectives', raci: 'PM' },
          { name: 'Stakeholder Analysis', type: 'feature', description: 'Stakeholder identification and analysis', raci: 'BA' },
          { name: 'Risk Assessment', type: 'feature', description: 'Initial risk identification', raci: 'PM' },
        ],
      },
      {
        name: 'Requirements',
        type: 'project',
        description: 'Requirements documentation',
        children: [
          { name: 'Business Requirements', type: 'feature', description: 'Business needs and requirements', raci: 'BA' },
          { name: 'Functional Requirements', type: 'feature', description: 'Functional specifications', raci: 'BA' },
          { name: 'Non-Functional Requirements', type: 'feature', description: 'NFR specifications', raci: 'Architect' },
        ],
      },
      {
        name: 'Design',
        type: 'project',
        description: 'Design documentation',
        children: [
          { name: 'Solution Design', type: 'feature', description: 'High-level solution design', raci: 'Architect' },
          { name: 'Technical Design', type: 'feature', description: 'Detailed technical design', raci: 'Tech Lead' },
          { name: 'Data Model', type: 'feature', description: 'Data structure definitions', raci: 'DBA' },
        ],
      },
      {
        name: 'Testing',
        type: 'project',
        description: 'Testing documentation',
        children: [
          { name: 'Test Strategy', type: 'feature', description: 'Overall testing approach', raci: 'QA Lead' },
          { name: 'Test Cases', type: 'feature', description: 'Detailed test scenarios', raci: 'QA' },
          { name: 'Test Results', type: 'feature', description: 'Test execution results', raci: 'QA' },
        ],
      },
      {
        name: 'Deployment',
        type: 'project',
        description: 'Deployment documentation',
        children: [
          { name: 'Deployment Plan', type: 'feature', description: 'Deployment procedures', raci: 'DevOps' },
          { name: 'Rollback Plan', type: 'feature', description: 'Rollback procedures', raci: 'DevOps' },
          { name: 'Post-Deployment', type: 'feature', description: 'Post-deployment validation', raci: 'Ops' },
        ],
      },
    ],
    metadata: {
      revisionControl: true,
      lifecycleTracking: true,
      approvalRequired: true,
      raciDefined: true,
      updateFrequency: 'project-based',
    },
  },

  // Feature Delivery / Development Project
  // Sprint/interval/phase milestone style documentation
  featureDelivery: {
    id: 'feature-delivery',
    name: 'Feature Delivery / Development Project',
    description: 'Sprint, interval, and phase milestone documentation structure with top-level structure and content',
    category: 'development',
    structure: [
      {
        name: 'Phase 1 Activities',
        type: 'area',
        description: 'Phase 1 deliverables',
        children: [
          {
            name: 'Feature A',
            type: 'feature',
            description: 'Feature A documentation',
            children: [
              { name: 'Requirements', type: 'feature', description: 'Feature requirements' },
              { name: 'Design', type: 'feature', description: 'Feature design' },
              { name: 'Implementation', type: 'feature', description: 'Implementation notes' },
              { name: 'Testing', type: 'feature', description: 'Test documentation' },
            ],
          },
          {
            name: 'Feature B',
            type: 'feature',
            description: 'Feature B documentation',
            children: [
              { name: 'Requirements', type: 'feature', description: 'Feature requirements' },
              { name: 'Design', type: 'feature', description: 'Feature design' },
              { name: 'Implementation', type: 'feature', description: 'Implementation notes' },
              { name: 'Testing', type: 'feature', description: 'Test documentation' },
            ],
          },
        ],
      },
      {
        name: 'Phase 2 Activities',
        type: 'area',
        description: 'Phase 2 deliverables',
        children: [
          {
            name: 'Feature C',
            type: 'feature',
            description: 'Feature C documentation',
            children: [
              { name: 'Requirements', type: 'feature', description: 'Feature requirements' },
              { name: 'Design', type: 'feature', description: 'Feature design' },
              { name: 'Implementation', type: 'feature', description: 'Implementation notes' },
              { name: 'Testing', type: 'feature', description: 'Test documentation' },
            ],
          },
        ],
      },
      {
        name: 'Sprint Documentation',
        type: 'area',
        description: 'Sprint-level documentation',
        children: [
          { name: 'Sprint Planning', type: 'feature', description: 'Sprint planning notes' },
          { name: 'Daily Standups', type: 'feature', description: 'Standup meeting notes' },
          { name: 'Sprint Review', type: 'feature', description: 'Sprint review documentation' },
          { name: 'Retrospective', type: 'feature', description: 'Sprint retrospective notes' },
        ],
      },
    ],
    metadata: {
      revisionControl: true,
      lifecycleTracking: true,
      approvalRequired: false,
      sprintBased: true,
      updateFrequency: 'sprint-based',
    },
  },
}

// Template categories for grouping
export const templateCategories = {
  operational: {
    name: 'Operational',
    description: 'Living documentation for ongoing operations',
    icon: 'settings',
    color: 'blue',
  },
  delivery: {
    name: 'Delivery',
    description: 'Project delivery documentation sets',
    icon: 'rocket_launch',
    color: 'green',
  },
  development: {
    name: 'Development',
    description: 'Feature development and sprint documentation',
    icon: 'code',
    color: 'purple',
  },
}

// Get all templates grouped by category
export function getTemplatesByCategory() {
  const grouped = {}
  Object.values(structureTemplates).forEach((template) => {
    const category = template.category
    if (!grouped[category]) {
      grouped[category] = {
        category: templateCategories[category],
        templates: [],
      }
    }
    grouped[category].templates.push(template)
  })
  return grouped
}

// Flatten template structure for creation
export function flattenTemplateStructure(template, parentId = null) {
  const structures = []
  let currentId = 0

  function processNode(node, parent = null) {
    const id = `template-${template.id}-${currentId++}`
    const structure = {
      id,
      name: node.name,
      type: node.type,
      description: node.description || '',
      parentId: parent,
      metadata: {
        ...node,
        templateId: template.id,
        templateName: template.name,
      },
    }
    
    if (node.raci) {
      structure.metadata.raci = node.raci
    }

    structures.push(structure)

    if (node.children && Array.isArray(node.children)) {
      node.children.forEach((child) => {
        processNode(child, id)
      })
    }
  }

  template.structure.forEach((root) => {
    processNode(root, parentId)
  })

  return structures
}

