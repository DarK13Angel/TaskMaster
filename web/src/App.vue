<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'

// Navigation state: 'overview', 'board', or 'project-summary'
const currentView = ref('overview')
const projectFilterTab = ref('active') // 'active' or 'archived'
const activeProjectId = ref('') // Selected project for project-summary view

// Three-dot popover dropdown menu state
const activeMenuProjectId = ref(null)

function toggleProjectMenu(id) {
  activeMenuProjectId.value = activeMenuProjectId.value === id ? null : id
}

function closeProjectMenu() {
  activeMenuProjectId.value = null
}

// Core Data
const projects = ref([])
const statusTags = ref([])
const tasks = ref([])
const priorities = ref(['low', 'medium', 'high', 'urgent'])

// Filters
const selectedProject = ref('')
const selectedPriority = ref('')
const searchQuery = ref('')

// Modals
const isTaskModalOpen = ref(false)
const isProjectModalOpen = ref(false)
const isEditModalOpen = ref(false)
const isProjectDetailModalOpen = ref(false)
const draggedTask = ref(null)

// Toast
const toast = reactive({
  message: '',
  type: 'info',
  visible: false
})

function showToast(message, type = 'info') {
  toast.message = message
  toast.type = type
  toast.visible = true
  setTimeout(() => {
    toast.visible = false
  }, 3500)
}

// New Task form state
const newTask = reactive({
  title: '',
  project_id: '',
  status_id: '',
  priority: 'medium',
  due_date: '',
  notes: ''
})

// Task Editor state
const editingTask = reactive({
  id: '',
  title: '',
  project_id: '',
  status_id: '',
  priority: 'medium',
  due_date: '',
  notes: '',
  subtasks: []
})
const newSubtaskTitle = ref('')

// Project Detail / Edit State
const projectDetail = reactive({
  id: '',
  name: '',
  description: '',
  is_archived: false,
  historical_done_count: 0,
  total: 0,
  done: 0,
  inProg: 0,
  pending: 0,
  totalLifetimeDone: 0,
  completionRate: 0,
  healthLabel: '',
  healthReason: ''
})

// New Project form state
const newProjectName = ref('')
const newProjectDesc = ref('')

// Retention & Cleanup
const retentionDays = ref(30)

async function triggerManualCleanup() {
  try {
    const res = await fetch(`/api/v1/system/cleanup?days=${retentionDays.value}`, { method: 'POST' })
    if (!res.ok) throw new Error('Cleanup failed')
    const data = await res.json()
    if (data.purged_count > 0) {
      showToast(`🧹 Purged ${data.purged_count} task(s) older than ${data.retention_days} days. Lifetime counts preserved on projects!`, 'info')
    } else {
      showToast(`No tasks older than ${data.retention_days} days to purge`, 'info')
    }
    await fetchProjects()
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Computed: Project Name of currently selected project
const selectedProjectName = computed(() => {
  if (!selectedProject.value) return 'All Projects'
  const p = projects.value.find(p => p.id === selectedProject.value)
  return p ? p.name : 'All Projects'
})

// Open New Task Modal with smart pre-selection
function openNewTaskModal(preselectedProjectId = null) {
  if (preselectedProjectId) {
    newTask.project_id = preselectedProjectId
  } else if (currentView.value === 'board' && selectedProject.value) {
    newTask.project_id = selectedProject.value
  } else if (currentView.value === 'project-summary' && activeProjectId.value) {
    newTask.project_id = activeProjectId.value
  } else {
    newTask.project_id = ''
  }

  const initial = statusTags.value.find(s => s.name === 'ToDo') || statusTags.value.find(s => s.is_initial)
  if (initial) newTask.status_id = initial.id

  newTask.title = ''
  newTask.notes = ''
  newTask.due_date = ''
  newTask.priority = 'medium'
  isTaskModalOpen.value = true
}

// Open Edit Task Modal
async function openEditModal(task) {
  try {
    const res = await fetch(`/api/v1/tasks/${task.id}`)
    if (!res.ok) throw new Error('Failed to load task details')
    const fullTask = await res.json()

    editingTask.id = fullTask.id
    editingTask.title = fullTask.title
    editingTask.project_id = fullTask.project_id
    editingTask.status_id = fullTask.status_id
    editingTask.priority = fullTask.priority
    editingTask.notes = fullTask.notes || ''
    editingTask.due_date = fullTask.due_date ? fullTask.due_date.split('T')[0] : ''
    editingTask.subtasks = fullTask.subtasks || []
    newSubtaskTitle.value = ''
    isEditModalOpen.value = true
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Open Project Detail & Summary Modal
function openProjectDetailModal(pSummary) {
  projectDetail.id = pSummary.id
  projectDetail.name = pSummary.name
  projectDetail.description = pSummary.description || ''
  projectDetail.is_archived = !!pSummary.is_archived
  projectDetail.historical_done_count = pSummary.historical_done_count || 0
  projectDetail.total = pSummary.total
  projectDetail.done = pSummary.done
  projectDetail.inProg = pSummary.inProg
  projectDetail.pending = pSummary.pending
  projectDetail.totalLifetimeDone = pSummary.totalLifetimeDone
  projectDetail.completionRate = pSummary.completionRate
  projectDetail.healthLabel = pSummary.healthLabel
  projectDetail.healthReason = pSummary.healthReason

  isProjectDetailModalOpen.value = true
}

// Open Project Summary Page (from 3-dot menu)
function openProjectSummaryPage(pSummary) {
  activeProjectId.value = pSummary.id
  currentView.value = 'project-summary'
  selectedProject.value = ''
  fetchTasks()
}

// Save Project Edits (Name, Description)
async function handleSaveProjectEdits() {
  if (!projectDetail.name.trim()) {
    showToast('Project name is required', 'error')
    return
  }

  try {
    const res = await fetch(`/api/v1/projects/${projectDetail.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: projectDetail.name.trim(),
        description: projectDetail.description.trim()
      })
    })

    if (!res.ok) throw new Error('Failed to update project')
    showToast('Project updated successfully', 'success')
    isProjectDetailModalOpen.value = false
    await fetchProjects()
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Toggle Project Archive
async function handleToggleProjectArchive() {
  const targetArchived = !projectDetail.is_archived
  try {
    const res = await fetch(`/api/v1/projects/${projectDetail.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_archived: targetArchived })
    })

    if (!res.ok) throw new Error('Failed to change archive status')
    projectDetail.is_archived = targetArchived
    showToast(targetArchived ? '📁 Project archived' : '📂 Project restored to active', 'info')
    isProjectDetailModalOpen.value = false
    if (targetArchived && selectedProject.value === projectDetail.id) {
      selectedProject.value = ''
      if (currentView.value === 'board') currentView.value = 'overview'
    }
    await fetchProjects()
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Delete Project
async function handleDeleteProject(projectId) {
  if (!confirm('Are you sure you want to delete this project? All associated tasks will be permanently removed!')) return

  try {
    const res = await fetch(`/api/v1/projects/${projectId}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete project')

    showToast('Project deleted', 'info')
    isProjectDetailModalOpen.value = false
    if (selectedProject.value === projectId) {
      selectedProject.value = ''
    }
    if (activeProjectId.value === projectId) {
      currentView.value = 'overview'
    }
    await fetchProjects()
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Save Edited Task
async function handleSaveTaskEdits() {
  if (!editingTask.title.trim()) {
    showToast('Task title is required', 'error')
    return
  }

  try {
    const payload = {
      title: editingTask.title.trim(),
      notes: editingTask.notes.trim(),
      priority: editingTask.priority,
      project_id: editingTask.project_id,
      due_date: editingTask.due_date ? new Date(editingTask.due_date).toISOString() : null
    }

    const res = await fetch(`/api/v1/tasks/${editingTask.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })

    if (!res.ok) {
      const err = await res.json()
      throw new Error(err.error || 'Failed to update task')
    }

    showToast('Task updated successfully!', 'success')
    isEditModalOpen.value = false
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Subtask: Add item
async function handleAddSubtask() {
  if (!newSubtaskTitle.value.trim() || !editingTask.id) return

  try {
    const res = await fetch(`/api/v1/tasks/${editingTask.id}/subtasks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: newSubtaskTitle.value.trim() })
    })

    if (!res.ok) throw new Error('Failed to add subtask')
    const created = await res.json()
    editingTask.subtasks.push(created)
    newSubtaskTitle.value = ''

    const t = tasks.value.find(t => t.id === editingTask.id)
    if (t) t.total_subtasks = (t.total_subtasks || 0) + 1
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Subtask: Toggle item
async function handleToggleSubtask(st) {
  const previousState = st.is_done
  st.is_done = !st.is_done

  try {
    const res = await fetch(`/api/v1/subtasks/${st.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_done: st.is_done })
    })

    if (!res.ok) {
      st.is_done = previousState
      throw new Error('Failed to toggle subtask')
    }

    const t = tasks.value.find(t => t.id === editingTask.id)
    if (t) {
      t.done_subtasks = (t.done_subtasks || 0) + (st.is_done ? 1 : -1)
    }
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Subtask: Delete item
async function handleDeleteSubtask(subtaskId) {
  try {
    const res = await fetch(`/api/v1/subtasks/${subtaskId}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete subtask')

    const idx = editingTask.subtasks.findIndex(s => s.id === subtaskId)
    if (idx !== -1) {
      const removed = editingTask.subtasks[idx]
      editingTask.subtasks.splice(idx, 1)

      const t = tasks.value.find(t => t.id === editingTask.id)
      if (t) {
        t.total_subtasks = Math.max(0, (t.total_subtasks || 1) - 1)
        if (removed.is_done) t.done_subtasks = Math.max(0, (t.done_subtasks || 1) - 1)
      }
    }
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Navigate directly to Project Board (Active projects only)
function openProjectBoard(projectId) {
  if (projectId) {
    const p = projects.value.find(p => p.id === projectId)
    if (p && p.is_archived) {
      showToast('Archived projects do not have an active board. Unarchive to view board.', 'info')
      return
    }
  }
  selectedProject.value = projectId
  currentView.value = 'board'
  fetchTasks()
}

// Fetch APIs
async function fetchProjects() {
  try {
    const res = await fetch('/api/v1/projects')
    if (res.ok) projects.value = await res.json()
  } catch (err) {
    showToast('Failed to load projects: ' + err.message, 'error')
  }
}

async function fetchStatusTags() {
  try {
    const res = await fetch('/api/v1/status-tags')
    if (res.ok) {
      statusTags.value = await res.json()
    }
  } catch (err) {
    showToast('Failed to load status tags', 'error')
  }
}

async function fetchTasks() {
  try {
    const params = new URLSearchParams()
    if (selectedProject.value && currentView.value === 'board') {
      params.append('project_id', selectedProject.value)
    }
    if (selectedPriority.value) params.append('priority', selectedPriority.value)
    if (searchQuery.value) params.append('search', searchQuery.value)

    const activeP = projects.value.find(p => p.id === activeProjectId.value)
    if (currentView.value === 'project-summary' && activeP && activeP.is_archived) {
      params.append('include_archived', 'true')
    }

    const res = await fetch(`/api/v1/tasks?${params.toString()}`)
    if (res.ok) {
      const data = await res.json()
      if (currentView.value === 'project-summary' && activeP && activeP.is_archived) {
        tasks.value = data
      } else {
        const activeProjectIds = new Set(projects.value.filter(p => !p.is_archived).map(p => p.id))
        tasks.value = data.filter(t => activeProjectIds.has(t.project_id))
      }
    }
  } catch (err) {
    showToast('Failed to load tasks', 'error')
  }
}

function tasksForStatus(statusId) {
  return tasks.value.filter(t => t.status_id === statusId)
}

// Metrics (Active Projects Only)
const metrics = computed(() => {
  const activeProjects = projects.value.filter(p => !p.is_archived)
  const activeProjectIds = new Set(activeProjects.map(p => p.id))

  const activeTasks = tasks.value.filter(t => activeProjectIds.has(t.project_id))
  const total = activeTasks.length
  const completed = activeTasks.filter(t => t.status_name === 'Done').length
  const killed = activeTasks.filter(t => t.status_name === 'Killed').length
  const inProgress = activeTasks.filter(t => t.status_name === 'In Progress').length
  const pending = activeTasks.filter(t => t.status_name === 'Backlog' || t.status_name === 'ToDo').length

  const totalHistoricalDone = activeProjects.reduce((acc, p) => acc + (p.historical_done_count || 0), 0)
  const lifetimeCompleted = completed + totalHistoricalDone
  const lifetimeTotal = total + totalHistoricalDone - killed
  const completionRate = lifetimeTotal > 0 ? Math.round((lifetimeCompleted / lifetimeTotal) * 100) : 0

  const openTasks = activeTasks.filter(t => t.status_name !== 'Done' && t.status_name !== 'Killed')
  let avgAgeDays = 0
  if (openTasks.length > 0) {
    const totalAgeMs = openTasks.reduce((sum, t) => sum + (Date.now() - new Date(t.created_at).getTime()), 0)
    avgAgeDays = (totalAgeMs / openTasks.length / (1000 * 60 * 60 * 24)).toFixed(1)
  }

  return {
    total,
    completed,
    lifetimeCompleted,
    killed,
    inProgress,
    pending,
    completionRate,
    avgAgeDays,
    openCount: openTasks.length
  }
})

// Focus tasks (Active Projects Only)
const focusTasks = computed(() => {
  const now = Date.now()
  const activeProjectIds = new Set(projects.value.filter(p => !p.is_archived).map(p => p.id))
  const openTasks = tasks.value.filter(t => activeProjectIds.has(t.project_id) && t.status_name !== 'Done' && t.status_name !== 'Killed')

  return openTasks
    .map(t => {
      const isOverdue = t.due_date ? new Date(t.due_date).getTime() < now : false
      const dueSoon = t.due_date ? (new Date(t.due_date).getTime() - now) < (48 * 60 * 60 * 1000) && !isOverdue : false
      return { ...t, isOverdue, dueSoon }
    })
    .filter(t => t.isOverdue || t.priority === 'urgent' || t.priority === 'high' || t.dueSoon)
    .sort((a, b) => {
      if (a.isOverdue && !b.isOverdue) return -1
      if (!a.isOverdue && b.isOverdue) return 1
      const pWeights = { urgent: 4, high: 3, medium: 2, low: 1 }
      return (pWeights[b.priority] || 0) - (pWeights[a.priority] || 0)
    })
    .slice(0, 5)
})

// Project summaries with Lifetime Done counts and health
const projectSummaries = computed(() => {
  const now = Date.now()

  return projects.value.map(p => {
    const pTasks = tasks.value.filter(t => t.project_id === p.id)
    const total = pTasks.length
    const done = pTasks.filter(t => t.status_name === 'Done').length
    const inProg = pTasks.filter(t => t.status_name === 'In Progress').length
    const pending = pTasks.filter(t => t.status_name === 'Backlog' || t.status_name === 'ToDo').length
    const overdue = pTasks.filter(t => t.status_name !== 'Done' && t.status_name !== 'Killed' && t.due_date && new Date(t.due_date).getTime() < now).length

    const historicalDone = p.historical_done_count || 0
    const totalLifetimeDone = historicalDone + done
    const totalLifetimeTasks = historicalDone + total
    const completionRate = totalLifetimeTasks > 0 ? Math.round((totalLifetimeDone / totalLifetimeTasks) * 100) : 0

    let health = 'on-track'
    let healthLabel = '🟢 On Track'
    let healthReason = 'Steady progress'

    if (p.is_archived) {
      health = 'archived'
      healthLabel = '📦 Archived'
      healthReason = 'Project archived'
    } else if (overdue > 0) {
      health = 'lagging'
      healthLabel = '🔴 Lagging'
      healthReason = `${overdue} overdue task${overdue > 1 ? 's' : ''}`
    } else if (pending > 3 && inProg === 0 && completionRate < 30) {
      health = 'attention'
      healthLabel = '🟡 Needs Attention'
      healthReason = 'High backlog, no active progress'
    } else if (completionRate === 100 && total > 0) {
      healthLabel = '✨ Completed'
      healthReason = 'All tasks finished'
    }

    return {
      ...p,
      total,
      done,
      inProg,
      pending,
      overdue,
      totalLifetimeDone,
      completionRate,
      health,
      healthLabel,
      healthReason
    }
  })
})

// Active Project Summary (for project-summary view)
const activeProjectSummary = computed(() => {
  return projectSummaries.value.find(p => p.id === activeProjectId.value) || {}
})

// Tasks belonging to the active project in summary view
const activeProjectTasks = computed(() => {
  return tasks.value.filter(t => t.project_id === activeProjectId.value)
})

// Filtered projects based on active / archived tab
const filteredProjectSummaries = computed(() => {
  if (projectFilterTab.value === 'archived') {
    return projectSummaries.value.filter(p => p.is_archived)
  }
  return projectSummaries.value.filter(p => !p.is_archived)
})

// Subtask percentage in Edit Modal
const subtaskStats = computed(() => {
  const total = editingTask.subtasks.length
  const done = editingTask.subtasks.filter(s => s.is_done).length
  const pct = total > 0 ? Math.round((done / total) * 100) : 0
  return { total, done, pct }
})

// Create Task
async function handleCreateTask() {
  if (!newTask.title.trim()) {
    showToast('Task title is required', 'error')
    return
  }
  if (!newTask.project_id) {
    showToast('Please select a project for this task', 'error')
    return
  }

  try {
    const payload = {
      title: newTask.title.trim(),
      project_id: newTask.project_id,
      status_id: newTask.status_id,
      priority: newTask.priority,
      notes: newTask.notes.trim(),
      due_date: newTask.due_date ? new Date(newTask.due_date).toISOString() : null
    }

    const res = await fetch('/api/v1/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })

    if (!res.ok) {
      const err = await res.json()
      throw new Error(err.error || 'Failed to create task')
    }

    showToast('Task created successfully!', 'success')
    isTaskModalOpen.value = false
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Create Project
async function handleCreateProject() {
  if (!newProjectName.value.trim()) return

  try {
    const res = await fetch('/api/v1/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newProjectName.value.trim(),
        description: newProjectDesc.value.trim()
      })
    })

    if (res.ok) {
      showToast('Project created!', 'success')
      isProjectModalOpen.value = false
      newProjectName.value = ''
      newProjectDesc.value = ''
      await fetchProjects()
      await fetchTasks()
    }
  } catch (err) {
    showToast('Failed to create project', 'error')
  }
}

// Move Task
async function moveTask(task, targetStatusId) {
  if (task.status_id === targetStatusId) return

  const oldStatusId = task.status_id
  task.status_id = targetStatusId

  try {
    const res = await fetch(`/api/v1/tasks/${task.id}/move`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target_status_id: targetStatusId })
    })

    if (!res.ok) {
      const err = await res.json()
      task.status_id = oldStatusId
      throw new Error(err.error || 'Move rejected')
    }

    const target = statusTags.value.find(s => s.id === targetStatusId)
    showToast(`Moved to ${target?.name || 'new status'}`, 'success')
    await fetchTasks()
  } catch (err) {
    showToast(err.message, 'error')
  }
}

// Delete Task
async function deleteTask(taskId) {
  if (!confirm('Are you sure you want to delete this task?')) return

  try {
    const res = await fetch(`/api/v1/tasks/${taskId}`, { method: 'DELETE' })
    if (res.ok) {
      tasks.value = tasks.value.filter(t => t.id !== taskId)
      showToast('Task deleted', 'info')
    }
  } catch (err) {
    showToast('Failed to delete task', 'error')
  }
}

// Drag & drop
function onDragStart(task) {
  draggedTask.value = task
}

function onDrop(targetStatus) {
  if (draggedTask.value && targetStatus) {
    moveTask(draggedTask.value, targetStatus.id)
    draggedTask.value = null
  }
}

// Format date
function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

onMounted(async () => {
  window.addEventListener('click', closeProjectMenu)
  await fetchProjects()
  await fetchStatusTags()
  await fetchTasks()
})

onUnmounted(() => {
  window.removeEventListener('click', closeProjectMenu)
})
</script>

<template>
  <div class="app-layout">
    <!-- Top Navigation Header -->
    <header class="header">
      <div class="header-left">
        <div class="logo" @click="currentView = 'overview'">
          <span class="logo-icon">⚡</span>
          <h1>TaskMaster</h1>
        </div>

        <!-- View Navigation Tabs (Clean tabs: no project name in navbar) -->
        <div class="view-tabs">
          <button 
            class="tab-btn" 
            :class="{ active: currentView === 'overview' }"
            @click="currentView = 'overview'; selectedProject = ''; fetchTasks()"
          >
            📊 Overview
          </button>
          <button 
            class="tab-btn" 
            :class="{ active: currentView === 'board' }"
            @click="currentView = 'board'; fetchTasks()"
          >
            📋 Board
          </button>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="header-right">
        <!-- New Project Button (Only on Overview page) -->
        <button v-if="currentView === 'overview'" class="btn-secondary" @click="isProjectModalOpen = true">
          <span>+</span> New Project
        </button>

        <!-- New Task Button -->
        <button class="btn-primary" @click="openNewTaskModal()">
          <span>+</span> New Task
        </button>
      </div>
    </header>

    <!-- ============================================== -->
    <!-- VIEW 1: HOMEPAGE / OVERVIEW METRICS DASHBOARD  -->
    <!-- ============================================== -->
    <main v-if="currentView === 'overview'" class="overview-container">
      <div class="overview-header">
        <div>
          <h2>Projects & Overview</h2>
          <p class="subtitle">Projects overview, priority radar, and team velocity</p>
        </div>
      </div>

      <!-- ROW 1 (FIRST): Projects Workspace Cards -->
      <section class="workspace-section">
        <div class="section-top-bar">
          <div class="section-title">
            <h3>📁 Projects Workspace</h3>
            <p>Click any card to open board. Press ⋮ to View project summary or Manage project</p>
          </div>

          <!-- Project Filter Tabs (Active vs Archived) -->
          <div class="project-tab-filter">
            <button 
              class="filter-pill" 
              :class="{ active: projectFilterTab === 'active' }"
              @click="projectFilterTab = 'active'"
            >
              Active ({{ projectSummaries.filter(p => !p.is_archived).length }})
            </button>
            <button 
              class="filter-pill" 
              :class="{ active: projectFilterTab === 'archived' }"
              @click="projectFilterTab = 'archived'"
            >
              📦 Archived ({{ projectSummaries.filter(p => p.is_archived).length }})
            </button>
          </div>
        </div>

        <div class="projects-grid">
          <div 
            v-for="p in filteredProjectSummaries" 
            :key="p.id" 
            class="project-card"
            :class="{ 'project-card-archived': p.is_archived }"
            @click="p.is_archived ? openProjectSummaryPage(p) : openProjectBoard(p.id)"
          >
            <div class="project-card-top">
              <h4>{{ p.name }}</h4>
              <div class="card-top-right">
                <span class="health-badge-sm" :class="'health-' + p.health">{{ p.healthLabel }}</span>
                <!-- Three-Dot Menu Button with Popover -->
                <div class="project-menu-container" @click.stop>
                  <button class="btn-dots" @click.stop="toggleProjectMenu(p.id)" title="Project options">⋮</button>
                  <div v-if="activeMenuProjectId === p.id" class="dropdown-popover" @click.stop>
                    <button class="dropdown-item" @click="openProjectSummaryPage(p); closeProjectMenu()">
                      <span class="dropdown-icon">📊</span> View Summary
                    </button>
                    <button class="dropdown-item" @click="openProjectDetailModal(p); closeProjectMenu()">
                      <span class="dropdown-icon">⚙️</span> Manage
                    </button>
                  </div>
                </div>
              </div>
            </div>
            <p v-if="p.description" class="project-desc">{{ p.description }}</p>
            <div class="progress-bar-bg">
              <div class="progress-bar-fill" :style="{ width: p.completionRate + '%' }"></div>
            </div>
            <div class="project-stats">
              <span class="text-neon">🏆 {{ p.totalLifetimeDone }} Done</span>
              <span>⚡ {{ p.inProg }} Active</span>
              <span>⏳ {{ p.pending }} Pending</span>
            </div>
            <div class="project-card-footer" @click.stop>
              <span class="card-rate-text">{{ p.completionRate }}% completed</span>
              <button v-if="!p.is_archived" class="btn-text" @click="openProjectBoard(p.id)">Open Board →</button>
              <button v-else class="btn-text-info" @click="openProjectSummaryPage(p)">View Summary →</button>
            </div>
          </div>

          <!-- Add Project Card (Active tab only) -->
          <div v-if="projectFilterTab === 'active'" class="project-card project-card-add" @click="isProjectModalOpen = true">
            <span class="add-icon">+</span>
            <h4>Create New Project</h4>
            <p>Setup a new board</p>
          </div>
        </div>
      </section>

      <!-- ROW 2 (SECOND): 4 High-Level Metric Cards -->
      <section class="metrics-grid">
        <!-- 1. Progress -->
        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">Progress</span>
            <span class="metric-icon">🎯</span>
          </div>
          <div class="metric-value">
            {{ metrics.lifetimeCompleted }} <span class="metric-denom">Tasks Done</span>
          </div>
          <div class="progress-bar-bg">
            <div class="progress-bar-fill" :style="{ width: metrics.completionRate + '%' }"></div>
          </div>
          <div class="metric-footer">
            <strong class="text-neon">{{ metrics.completionRate }}%</strong> overall completion
          </div>
        </div>

        <!-- 2. Pending Work -->
        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">Pending Queue</span>
            <span class="metric-icon">⏳</span>
          </div>
          <div class="metric-value">{{ metrics.pending }} <span class="metric-denom">Tasks</span></div>
          <div class="metric-subtext">Waiting in Backlog & ToDo</div>
          <div class="metric-footer text-muted">Awaiting active sprint</div>
        </div>

        <!-- 3. In Progress -->
        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">In Progress</span>
            <span class="metric-icon">🔥</span>
          </div>
          <div class="metric-value text-neon">{{ metrics.inProgress }} <span class="metric-denom">Active</span></div>
          <div class="metric-subtext">Currently being worked on</div>
          <div class="metric-footer text-neon">Primary focus workload</div>
        </div>

        <!-- 4. Average Task Age -->
        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">Average Task Age</span>
            <span class="metric-icon">⏱️</span>
          </div>
          <div class="metric-value">{{ metrics.avgAgeDays }} <span class="metric-denom">Days</span></div>
          <div class="metric-subtext">Created to active duration</div>
          <div class="metric-footer" :class="metrics.avgAgeDays > 7 ? 'text-red' : 'text-green'">
            {{ metrics.avgAgeDays > 7 ? '⚠️ Bottleneck detected' : '🟢 Healthy velocity' }}
          </div>
        </div>
      </section>

      <!-- ROW 3 (THIRD): Focus Radar & Project Health -->
      <section class="diagnostic-grid">
        <!-- Focus Radar -->
        <div class="panel">
          <div class="panel-header">
            <h3>🔥 What to Focus On Next</h3>
            <span class="badge-count">{{ focusTasks.length }} Critical</span>
          </div>
          <div class="panel-body">
            <div v-if="focusTasks.length === 0" class="empty-state">
              🎉 No urgent or overdue tasks! You're on track.
            </div>
            <div 
              v-for="task in focusTasks" 
              :key="task.id" 
              class="focus-item"
              @click="openProjectBoard(task.project_id)"
            >
              <div class="focus-item-left">
                <span class="badge" :class="'badge-' + task.priority">{{ task.priority }}</span>
                <div>
                  <div class="focus-title">{{ task.title }}</div>
                  <div class="focus-meta">📁 {{ task.project_name }} • {{ task.status_name }}</div>
                </div>
              </div>
              <div class="focus-item-right">
                <span v-if="task.isOverdue" class="overdue-tag">⚠️ Overdue</span>
                <span v-else-if="task.due_date" class="due-tag">📅 {{ formatDate(task.due_date) }}</span>
                <span class="arrow-icon">→</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Project Lagging Diagnostics -->
        <div class="panel">
          <div class="panel-header">
            <h3>⚠️ Project Health & Lagging Indicators</h3>
            <span class="badge-count">{{ projectSummaries.filter(p => !p.is_archived).length }} Active</span>
          </div>
          <div class="panel-body">
            <div v-if="projectSummaries.filter(p => !p.is_archived).length === 0" class="empty-state">
              No active projects created yet.
            </div>
            <div 
              v-for="p in projectSummaries.filter(p => !p.is_archived)" 
              :key="p.id" 
              class="health-item"
              @click="openProjectBoard(p.id)"
            >
              <div class="health-item-header">
                <div class="health-name">{{ p.name }}</div>
                <div style="display:flex; align-items:center; gap:8px;">
                  <span class="health-badge" :class="'health-' + p.health">{{ p.healthLabel }}</span>
                  <!-- Three-Dot Menu Button with Popover -->
                  <div class="project-menu-container" @click.stop>
                    <button class="btn-dots" @click.stop="toggleProjectMenu('diag-' + p.id)" title="Project options">⋮</button>
                    <div v-if="activeMenuProjectId === 'diag-' + p.id" class="dropdown-popover" @click.stop>
                      <button class="dropdown-item" @click="openProjectSummaryPage(p); closeProjectMenu()">
                        <span class="dropdown-icon">📊</span> View
                      </button>
                      <button class="dropdown-item" @click="openProjectDetailModal(p); closeProjectMenu()">
                        <span class="dropdown-icon">⚙️</span> Manage
                      </button>
                    </div>
                  </div>
                </div>
              </div>
              <div class="progress-bar-bg-sm">
                <div class="progress-bar-fill" :style="{ width: p.completionRate + '%' }"></div>
              </div>
              <div class="health-meta">
                <span>{{ p.completionRate }}% done ({{ p.totalLifetimeDone }} lifetime done)</span>
                <span class="health-reason">{{ p.healthReason }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- ============================================== -->
    <!-- VIEW 2: DEDICATED PROJECT SUMMARY PAGE         -->
    <!-- ============================================== -->
    <main v-else-if="currentView === 'project-summary'" class="project-summary-page">
      <div class="summary-top-nav">
        <button class="btn-back" @click="currentView = 'overview'">← Back to Overview</button>
        <button v-if="!activeProjectSummary.is_archived" class="btn-secondary btn-sm" @click="openProjectBoard(activeProjectSummary.id)">📋 View Board →</button>
        <span v-else class="health-badge health-archived">📦 Archived Project (Board Disabled)</span>
      </div>

      <div class="summary-page-header">
        <div class="summary-title-area">
          <div class="summary-title-row">
            <h2>{{ activeProjectSummary.name }}</h2>
            <span class="health-badge" :class="'health-' + activeProjectSummary.health">{{ activeProjectSummary.healthLabel }}</span>
          </div>
          <p class="summary-desc-text">
            {{ activeProjectSummary.description || 'No description provided for this project.' }}
          </p>
        </div>

        <!-- Action Icons & Buttons in Header -->
        <div class="summary-header-actions">
          <button class="btn-secondary" @click="openProjectDetailModal(activeProjectSummary)" title="Edit and manage project details">
            ✏️ Manage Project
          </button>
          <button v-if="!activeProjectSummary.is_archived" class="btn-primary" @click="openProjectBoard(activeProjectSummary.id)" title="View Kanban Board Dashboard">
            📋 View Board →
          </button>
        </div>
      </div>

      <!-- Summary of that project only -->
      <section class="metrics-grid">
        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">Lifetime Tasks Done</span>
            <span class="metric-icon">🏆</span>
          </div>
          <div class="metric-value text-neon">{{ activeProjectSummary.totalLifetimeDone }}</div>
          <div class="progress-bar-bg-sm">
            <div class="progress-bar-fill" :style="{ width: activeProjectSummary.completionRate + '%' }"></div>
          </div>
          <div class="metric-footer">{{ activeProjectSummary.completionRate }}% project completion rate</div>
        </div>

        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">In Progress (Active)</span>
            <span class="metric-icon">⚡</span>
          </div>
          <div class="metric-value text-blue">{{ activeProjectSummary.inProg }}</div>
          <div class="metric-subtext">Tasks currently in execution</div>
          <div class="metric-footer text-blue">Active focus</div>
        </div>

        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">Pending Queue</span>
            <span class="metric-icon">⏳</span>
          </div>
          <div class="metric-value">{{ activeProjectSummary.pending }}</div>
          <div class="metric-subtext">Waiting in Backlog & ToDo</div>
          <div class="metric-footer text-muted">Upcoming backlog</div>
        </div>

        <div class="metric-card">
          <div class="metric-top">
            <span class="metric-label">Project Health</span>
            <span class="metric-icon">🩺</span>
          </div>
          <div class="metric-value" :class="activeProjectSummary.health === 'lagging' ? 'text-red' : 'text-green'">
            {{ activeProjectSummary.healthLabel }}
          </div>
          <div class="metric-subtext">{{ activeProjectSummary.healthReason }}</div>
          <div class="metric-footer">{{ activeProjectSummary.overdue }} overdue task(s)</div>
        </div>
      </section>

      <!-- Tasks in this Project Preview -->
      <section class="project-tasks-preview">
        <div class="preview-header">
          <h3>Tasks in {{ activeProjectSummary.name }} ({{ activeProjectTasks.length }})</h3>
          <div style="display:flex; gap:10px;">
            <button v-if="!activeProjectSummary.is_archived" class="btn-secondary btn-sm" @click="openProjectBoard(activeProjectSummary.id)">📋 View Board</button>
            <button v-if="!activeProjectSummary.is_archived" class="btn-primary btn-sm" @click="openNewTaskModal(activeProjectSummary.id)">+ New Task</button>
          </div>
        </div>

        <div class="tasks-table">
          <div 
            v-for="t in activeProjectTasks" 
            :key="t.id" 
            class="task-row" 
            @click="openEditModal(t)"
          >
            <div class="task-row-left">
              <span class="badge" :class="'badge-' + t.priority">{{ t.priority }}</span>
              <span class="task-row-title">{{ t.title }}</span>
              <span v-if="t.total_subtasks > 0" class="subtask-mini-pill">☑️ {{ t.done_subtasks }}/{{ t.total_subtasks }}</span>
            </div>
            <div class="task-row-right">
              <span class="status-pill">{{ t.status_name }}</span>
              <span v-if="t.due_date" class="due-tag">📅 {{ formatDate(t.due_date) }}</span>
            </div>
          </div>

          <div v-if="activeProjectTasks.length === 0" class="empty-state">
            <p style="margin-bottom: 10px;">No tasks in this project yet.</p>
            <div v-if="!activeProjectSummary.is_archived" style="display: flex; justify-content: center; gap: 10px;">
              <button class="btn-secondary btn-sm" @click="openProjectBoard(activeProjectSummary.id)">📋 View Board</button>
              <button class="btn-primary btn-sm" @click="openNewTaskModal(activeProjectSummary.id)">+ New Task</button>
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- ============================================== -->
    <!-- VIEW 2: KANBAN BOARD VIEW                      -->
    <!-- ============================================== -->
    <div v-else class="board-view-wrapper">
      <!-- Board Sub-Toolbar (Project selector, Priority filter, Search) -->
      <div class="board-toolbar">
        <div class="toolbar-left">
          <div class="project-picker">
            <label>Project:</label>
            <select v-model="selectedProject" @change="fetchTasks">
              <option value="">All Projects</option>
              <option v-for="p in projects.filter(p => !p.is_archived)" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>
        </div>

        <div class="toolbar-right">
          <div class="search-box">
            <input 
              type="text" 
              v-model="searchQuery" 
              @input="fetchTasks" 
              placeholder="Search tasks..." 
            />
          </div>

          <select v-model="selectedPriority" @change="fetchTasks" class="priority-select">
            <option value="">All Priorities</option>
            <option value="urgent">Urgent</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
        </div>
      </div>

      <main class="board-container">
      <div 
        v-for="col in statusTags" 
        :key="col.id" 
        class="column"
        :class="{ 'col-done': col.name === 'Done', 'col-killed': col.name === 'Killed' }"
        @dragover.prevent
        @drop="onDrop(col)"
      >
        <div class="column-header">
          <div class="column-title-group">
            <span class="column-dot" :class="'dot-' + col.name.toLowerCase().replace(/\s+/g, '-')"></span>
            <h3>{{ col.name }}</h3>
          </div>
          <span class="task-count">{{ tasksForStatus(col.id).length }}</span>
        </div>

        <!-- Task Cards List -->
        <div class="cards-list">
          <div 
            v-for="task in tasksForStatus(col.id)" 
            :key="task.id" 
            class="card"
            draggable="true"
            @dragstart="onDragStart(task)"
            @click="openEditModal(task)"
          >
            <div class="card-top">
              <span class="badge" :class="'badge-' + task.priority">
                {{ task.priority }}
              </span>
              <button class="btn-card-delete" @click.stop="deleteTask(task.id)" title="Delete Task">×</button>
            </div>

            <h4 class="card-title">{{ task.title }}</h4>
            <p v-if="task.notes" class="card-notes">{{ task.notes }}</p>

            <!-- Subtask Checklist Indicator on Card -->
            <div v-if="task.total_subtasks > 0" class="card-checklist-badge">
              <div class="checklist-info">
                <span>☑️ {{ task.done_subtasks }}/{{ task.total_subtasks }}</span>
                <span>{{ Math.round((task.done_subtasks / task.total_subtasks) * 100) }}%</span>
              </div>
              <div class="checklist-progress-bar">
                <div class="checklist-progress-fill" :style="{ width: ((task.done_subtasks / task.total_subtasks) * 100) + '%' }"></div>
              </div>
            </div>

            <div class="card-footer">
              <span v-if="task.project_name" class="project-tag">
                📁 {{ task.project_name }}
              </span>
              <span v-if="task.due_date" class="due-tag">
                📅 {{ formatDate(task.due_date) }}
              </span>
            </div>

            <!-- Quick Move Dropdown -->
            <div class="card-move" @click.stop>
              <label>Move:</label>
              <select :value="task.status_id" @change="moveTask(task, $event.target.value)">
                <option v-for="s in statusTags" :key="s.id" :value="s.id">{{ s.name }}</option>
              </select>
            </div>
          </div>

          <div v-if="tasksForStatus(col.id).length === 0" class="empty-dropzone">
            Drop task here
          </div>
        </div>
      </div>
    </main>
  </div>

    <!-- Toast Notification -->
    <div v-if="toast.visible" class="toast" :class="'toast-' + toast.type">
      {{ toast.message }}
    </div>

    <!-- Modal: New Task -->
    <div v-if="isTaskModalOpen" class="modal-overlay" @click.self="isTaskModalOpen = false">
      <div class="modal">
        <div class="modal-header">
          <h2>Create New Task</h2>
          <button class="btn-close" @click="isTaskModalOpen = false">×</button>
        </div>
        <form @submit.prevent="handleCreateTask" class="modal-body">
          <div class="form-group">
            <label>Task Title *</label>
            <input type="text" v-model="newTask.title" placeholder="e.g. Implement user auth" required autofocus />
          </div>

          <div class="form-row">
            <div class="form-group">
              <label>Project *</label>
              <select v-model="newTask.project_id" required>
                <option value="" disabled>-- Select a project --</option>
                <option v-for="p in projects.filter(p => !p.is_archived)" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </div>

            <div class="form-group">
              <label>Initial Status</label>
              <select v-model="newTask.status_id">
                <option v-for="s in statusTags.filter(t => t.is_initial)" :key="s.id" :value="s.id">
                  {{ s.name }}
                </option>
              </select>
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label>Priority</label>
              <select v-model="newTask.priority">
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
                <option value="urgent">Urgent</option>
              </select>
            </div>

            <div class="form-group">
              <label>Due Date (optional)</label>
              <input type="date" v-model="newTask.due_date" />
            </div>
          </div>

          <div class="form-group">
            <label>Notes / Description</label>
            <textarea v-model="newTask.notes" rows="3" placeholder="Optional notes..."></textarea>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="isTaskModalOpen = false">Cancel</button>
            <button type="submit" class="btn-primary">Create Task</button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal: Task Inspector & Checklist Editor -->
    <div v-if="isEditModalOpen" class="modal-overlay" @click.self="isEditModalOpen = false">
      <div class="modal modal-lg">
        <div class="modal-header">
          <h2>Task Inspector</h2>
          <button class="btn-close" @click="isEditModalOpen = false">×</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>Title *</label>
            <input type="text" v-model="editingTask.title" required />
          </div>

          <div class="form-row">
            <div class="form-group">
              <label>Project</label>
              <select v-model="editingTask.project_id">
                <option v-for="p in projects.filter(p => !p.is_archived)" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </div>

            <div class="form-group">
              <label>Priority</label>
              <select v-model="editingTask.priority">
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
                <option value="urgent">Urgent</option>
              </select>
            </div>

            <div class="form-group">
              <label>Due Date</label>
              <input type="date" v-model="editingTask.due_date" />
            </div>
          </div>

          <div class="form-group">
            <label>Notes / Details</label>
            <textarea v-model="editingTask.notes" rows="3" placeholder="Add descriptions, markdown, or notes..."></textarea>
          </div>

          <!-- CHECKLIST / SUBTASKS SECTION -->
          <div class="subtasks-section">
            <div class="subtasks-header">
              <label>☑️ To-Do Checklist</label>
              <span v-if="subtaskStats.total > 0" class="subtask-progress-label">
                {{ subtaskStats.done }} of {{ subtaskStats.total }} completed ({{ subtaskStats.pct }}%)
              </span>
            </div>

            <!-- Checklist Progress Bar -->
            <div v-if="subtaskStats.total > 0" class="progress-bar-bg-sm">
              <div class="progress-bar-fill" :style="{ width: subtaskStats.pct + '%' }"></div>
            </div>

            <!-- Subtask Items List -->
            <div class="subtask-list">
              <div 
                v-for="st in editingTask.subtasks" 
                :key="st.id" 
                class="subtask-item"
                :class="{ 'subtask-done': st.is_done }"
              >
                <input 
                  type="checkbox" 
                  :checked="st.is_done" 
                  @change="handleToggleSubtask(st)"
                  class="subtask-checkbox" 
                />
                <span class="subtask-title" @click="handleToggleSubtask(st)">{{ st.title }}</span>
                <button class="btn-subtask-del" @click="handleDeleteSubtask(st.id)" title="Delete Item">×</button>
              </div>
            </div>

            <!-- Add Subtask Input -->
            <div class="add-subtask-box">
              <input 
                type="text" 
                v-model="newSubtaskTitle" 
                @keyup.enter="handleAddSubtask"
                placeholder="Add checklist item (press Enter)..." 
              />
              <button class="btn-secondary btn-sm" @click="handleAddSubtask">+ Add</button>
            </div>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="isEditModalOpen = false">Close</button>
            <button type="button" class="btn-primary" @click="handleSaveTaskEdits">Save Changes</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: Project Detail & Summary Inspector -->
    <div v-if="isProjectDetailModalOpen" class="modal-overlay" @click.self="isProjectDetailModalOpen = false">
      <div class="modal modal-lg">
        <div class="modal-header">
          <h2>Project Settings & Info</h2>
          <button class="btn-close" @click="isProjectDetailModalOpen = false">×</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>Project Name *</label>
            <input type="text" v-model="projectDetail.name" required />
          </div>

          <div class="form-group">
            <label>Description</label>
            <textarea v-model="projectDetail.description" rows="2" placeholder="Project goals and summary..."></textarea>
          </div>

          <!-- Project Lifetime Statistics -->
          <div class="subtasks-section">
            <div class="subtasks-header">
              <label>📊 Project Lifetime Statistics</label>
              <span class="health-badge" :class="'health-' + (projectDetail.is_archived ? 'archived' : 'on-track')">
                {{ projectDetail.healthLabel }}
              </span>
            </div>

            <div class="project-stat-cards">
              <div class="stat-box">
                <span class="stat-number text-neon">{{ projectDetail.totalLifetimeDone }}</span>
                <span class="stat-title">Lifetime Tasks Done</span>
                <span class="stat-sub">{{ projectDetail.totalLifetimeDone }} completed tasks</span>
              </div>
              <div class="stat-box">
                <span class="stat-number text-blue">{{ projectDetail.inProg }}</span>
                <span class="stat-title">In Progress Active</span>
                <span class="stat-sub">Current focus work</span>
              </div>
              <div class="stat-box">
                <span class="stat-number text-muted">{{ projectDetail.pending }}</span>
                <span class="stat-title">Pending Tasks</span>
                <span class="stat-sub">In Backlog & ToDo</span>
              </div>
            </div>

            <div class="progress-bar-bg" style="margin-top: 14px;">
              <div class="progress-bar-fill" :style="{ width: projectDetail.completionRate + '%' }"></div>
            </div>
            <div style="font-size: 12px; color: #94a3b8; display: flex; justify-content: space-between; margin-top: 4px;">
              <span>Overall Completion Rate: <strong>{{ projectDetail.completionRate }}%</strong></span>
              <span>Total Active Tasks: {{ projectDetail.total }}</span>
            </div>
          </div>

          <div class="project-modal-footer">
            <div class="footer-left">
              <button 
                type="button" 
                class="btn-archive" 
                @click="handleToggleProjectArchive"
              >
                {{ projectDetail.is_archived ? '📂 Unarchive Project' : '📦 Archive Project' }}
              </button>
              <button 
                type="button" 
                class="btn-danger" 
                @click="handleDeleteProject(projectDetail.id)"
              >
                🗑️ Delete Project
              </button>
            </div>

            <div class="footer-right">
              <button 
                v-if="!projectDetail.is_archived" 
                type="button" 
                class="btn-primary" 
                @click="openProjectBoard(projectDetail.id); isProjectDetailModalOpen = false"
              >
                Open Board →
              </button>
              <button type="button" class="btn-secondary" @click="handleSaveProjectEdits">
                Save Info
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal: New Project -->
    <div v-if="isProjectModalOpen" class="modal-overlay" @click.self="isProjectModalOpen = false">
      <div class="modal">
        <div class="modal-header">
          <h2>Create New Project</h2>
          <button class="btn-close" @click="isProjectModalOpen = false">×</button>
        </div>
        <form @submit.prevent="handleCreateProject" class="modal-body">
          <div class="form-group">
            <label>Project Name *</label>
            <input type="text" v-model="newProjectName" placeholder="e.g. Website Redesign" required />
          </div>
          <div class="form-group">
            <label>Description</label>
            <textarea v-model="newProjectDesc" rows="2" placeholder="Brief project summary..."></textarea>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="isProjectModalOpen = false">Cancel</button>
            <button type="submit" class="btn-primary">Save Project</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Dark Slate Theme with Neon Blue Outlines */
.app-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
  background-color: #0b0f19;
  color: #f1f5f9;
}

/* Header */
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 11px 24px;
  background: #111827;
  border-bottom: 1px solid rgba(59, 130, 246, 0.25);
  box-shadow: 0 4px 20px -2px rgba(0, 0, 0, 0.5), 0 0 15px rgba(59, 130, 246, 0.08);
}

.header-left, .header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  text-shadow: 0 0 12px rgba(59, 130, 246, 0.5);
}

.logo-icon {
  font-size: 22px;
  filter: drop-shadow(0 0 8px #38bdf8);
}

.logo h1 {
  font-size: 19px;
  font-weight: 800;
  letter-spacing: -0.3px;
  background: linear-gradient(135deg, #ffffff 40%, #38bdf8 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* View Tabs Switcher */
.view-tabs {
  display: flex;
  background: #1a2234;
  padding: 3px;
  border-radius: 8px;
  margin-left: 8px;
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.tab-btn {
  background: transparent;
  border: none;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #94a3b8;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-btn:hover { color: #38bdf8; }

.tab-btn.active {
  background: #0f172a;
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.4);
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.2);
}

.project-picker {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 8px;
  font-size: 13px;
  color: #94a3b8;
}

.project-picker select, .priority-select, .search-box input {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid rgba(59, 130, 246, 0.25);
  background: #161f32;
  color: #f1f5f9;
  font-size: 13px;
  outline: none;
  transition: all 0.2s;
}

.project-picker select:focus, .priority-select:focus, .search-box input:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.4);
}

.search-box input { width: 180px; }

/* Neon Buttons */
.btn-primary {
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  color: #ffffff;
  border: 1px solid rgba(56, 189, 248, 0.5);
  padding: 7px 16px;
  border-radius: 7px;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  box-shadow: 0 0 12px rgba(37, 99, 235, 0.4);
  transition: all 0.2s;
}

.btn-primary:hover {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  box-shadow: 0 0 16px rgba(56, 189, 248, 0.6);
  transform: translateY(-1px);
}

.btn-secondary {
  background: #1a2234;
  color: #cbd5e1;
  border: 1px solid rgba(59, 130, 246, 0.3);
  padding: 7px 14px;
  border-radius: 7px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-secondary:hover {
  border-color: #38bdf8;
  color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.25);
}

.btn-archive {
  background: #1e293b;
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.btn-danger {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
}

/* ======================================================== */
/* OVERVIEW DASHBOARD STYLES                                */
/* ======================================================== */
.overview-container {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
  background: #0b0f19;
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.overview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.overview-header h2 {
  font-size: 24px;
  font-weight: 800;
  color: #f8fafc;
  letter-spacing: -0.3px;
}

.subtitle { color: #94a3b8; font-size: 14px; margin-top: 4px; }

.retention-badge {
  display: flex;
  align-items: center;
  gap: 10px;
  background: #131b2a;
  border: 1px solid rgba(59, 130, 246, 0.22);
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  color: #94a3b8;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.08);
}

.btn-text-sm {
  background: none;
  border: none;
  color: #38bdf8;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.btn-text-sm:hover {
  text-decoration: underline;
  text-shadow: 0 0 8px rgba(56, 189, 248, 0.4);
}

/* Metrics Grid */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.metric-card {
  background: #131b2a;
  border: 1px solid rgba(59, 130, 246, 0.22);
  border-radius: 12px;
  padding: 18px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  transition: all 0.2s;
}

.metric-card:hover {
  border-color: rgba(56, 189, 248, 0.45);
  box-shadow: 0 0 14px rgba(56, 189, 248, 0.2);
  transform: translateY(-2px);
}

.metric-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.metric-label { font-size: 13px; font-weight: 600; color: #94a3b8; }
.metric-icon { font-size: 18px; }

.metric-value {
  font-size: 26px;
  font-weight: 800;
  color: #f8fafc;
  margin-bottom: 6px;
}

.metric-denom { font-size: 14px; font-weight: 500; color: #64748b; }
.metric-subtext { font-size: 12px; color: #94a3b8; margin-bottom: 8px; }
.metric-footer { font-size: 12px; font-weight: 600; color: #cbd5e1; }

.text-neon { color: #38bdf8 !important; text-shadow: 0 0 8px rgba(56, 189, 248, 0.4); }
.text-blue { color: #60a5fa !important; }
.text-green { color: #4ade80 !important; }
.text-red { color: #f87171 !important; }
.text-muted { color: #64748b !important; }

/* Progress Bars */
.progress-bar-bg {
  background: #1e293b;
  height: 8px;
  border-radius: 4px;
  overflow: hidden;
  margin: 10px 0;
  border: 1px solid rgba(59, 130, 246, 0.15);
}

.progress-bar-bg-sm {
  background: #1e293b;
  height: 6px;
  border-radius: 3px;
  overflow: hidden;
  margin: 6px 0;
}

.progress-bar-fill {
  background: linear-gradient(90deg, #2563eb, #38bdf8);
  height: 100%;
  border-radius: 4px;
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.5);
  transition: width 0.3s ease;
}

/* Diagnostic Grid */
.diagnostic-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 900px) {
  .diagnostic-grid { grid-template-columns: 1fr; }
}

.panel {
  background: #131b2a;
  border: 1px solid rgba(59, 130, 246, 0.22);
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(59, 130, 246, 0.15);
}

.panel-header h3 { font-size: 16px; font-weight: 700; color: #f8fafc; }

.badge-count {
  background: #1e293b;
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
  font-size: 12px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 12px;
}

.panel-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.empty-state {
  text-align: center;
  padding: 24px;
  color: #64748b;
  font-size: 13px;
}

/* Focus Item */
.focus-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  border-radius: 8px;
  border: 1px solid rgba(59, 130, 246, 0.2);
  background: #162033;
  cursor: pointer;
  transition: all 0.2s;
}

.focus-item:hover {
  background: #1c2842;
  border-color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.2);
  transform: translateX(2px);
}

.focus-item-left { display: flex; align-items: center; gap: 12px; }
.focus-title { font-size: 13px; font-weight: 600; color: #f1f5f9; }
.focus-meta { font-size: 11px; color: #94a3b8; margin-top: 2px; }

.focus-item-right { display: flex; align-items: center; gap: 10px; }
.arrow-icon { color: #38bdf8; font-weight: 700; }

.overdue-tag {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
  font-size: 11px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 6px;
}

/* Project Health Item */
.health-item {
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid rgba(59, 130, 246, 0.2);
  background: #162033;
  cursor: pointer;
  transition: all 0.2s;
}

.health-item:hover {
  background: #1c2842;
  border-color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.2);
}

.health-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.health-name { font-size: 14px; font-weight: 700; color: #f1f5f9; }

.health-badge {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
}

.health-lagging { background: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }
.health-attention { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.health-on-track { background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
.health-archived { background: rgba(148, 163, 184, 0.15); color: #94a3b8; border: 1px solid rgba(148, 163, 184, 0.3); }

.health-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #94a3b8;
  margin-top: 6px;
}

.health-reason { font-weight: 600; }

/* Three-Dot Menu Button */
.btn-dots {
  background: transparent;
  border: none;
  font-size: 18px;
  color: #94a3b8;
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
  transition: all 0.2s;
}

.btn-dots:hover {
  color: #38bdf8;
  text-shadow: 0 0 8px rgba(56, 189, 248, 0.5);
  transform: scale(1.15);
}

.card-top-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Three-dot dropdown menu */
.project-menu-container {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.dropdown-popover {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 6px;
  background: #111827;
  border: 1px solid rgba(56, 189, 248, 0.45);
  border-radius: 8px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.8), 0 0 16px rgba(56, 189, 248, 0.25);
  min-width: 160px;
  z-index: 70;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.dropdown-item {
  background: transparent;
  border: none;
  color: #e2e8f0;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}

.dropdown-item:hover {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
}

.dropdown-icon {
  font-size: 14px;
}

.card-rate-text {
  font-size: 12px;
  color: #94a3b8;
  font-weight: 600;
}

/* Projects Workspace Section */
.workspace-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.section-top-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}

.section-title h3 { font-size: 17px; font-weight: 700; color: #f8fafc; }
.section-title p { font-size: 13px; color: #94a3b8; }

.project-tab-filter {
  display: flex;
  background: #131b2a;
  border: 1px solid rgba(59, 130, 246, 0.2);
  padding: 3px;
  border-radius: 8px;
  gap: 4px;
}

.filter-pill {
  background: transparent;
  border: none;
  color: #94a3b8;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.filter-pill.active {
  background: #1e293b;
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
}

.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.project-card {
  background: #131b2a;
  border: 1px solid rgba(59, 130, 246, 0.22);
  border-radius: 12px;
  padding: 18px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s, border-color 0.2s;
  display: flex;
  flex-direction: column;
}

.project-card:hover {
  transform: translateY(-2px);
  border-color: #38bdf8;
  box-shadow: 0 0 16px rgba(56, 189, 248, 0.25);
}

.project-card-archived {
  opacity: 0.75;
  border-style: dashed;
}

.project-card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.project-card-top h4 { font-size: 15px; font-weight: 700; color: #f8fafc; }

.health-badge-sm {
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
}

.project-desc {
  font-size: 12px;
  color: #94a3b8;
  margin-bottom: 12px;
}

.project-stats {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #94a3b8;
  font-weight: 600;
  margin-top: 8px;
}

.project-card-footer {
  margin-top: 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.btn-text-info {
  background: none;
  border: none;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
}
.btn-text-info:hover { color: #f1f5f9; text-decoration: underline; }

.btn-text {
  background: none;
  border: none;
  color: #38bdf8;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}
.btn-text:hover { text-shadow: 0 0 8px rgba(56, 189, 248, 0.4); }

.project-card-add {
  border: 2px dashed rgba(59, 130, 246, 0.35);
  background: rgba(19, 27, 42, 0.5);
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 28px;
}

.project-card-add:hover {
  border-color: #38bdf8;
  background: rgba(30, 41, 64, 0.5);
  box-shadow: 0 0 16px rgba(56, 189, 248, 0.2);
}

.add-icon { font-size: 28px; color: #38bdf8; margin-bottom: 6px; font-weight: 300; }

/* ======================================================== */
/* VIEW 2: DEDICATED PROJECT SUMMARY PAGE STYLES           */
/* ======================================================== */
.project-summary-page {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
  background: #0b0f19;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.summary-top-nav {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: -4px;
}

.btn-back {
  background: none;
  border: none;
  color: #38bdf8;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.btn-back:hover { text-decoration: underline; }

.summary-page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  background: #111827;
  border: 1px solid rgba(59, 130, 246, 0.25);
  border-radius: 12px;
  padding: 20px 24px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.4);
}

.summary-title-area {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.summary-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.summary-title-row h2 {
  font-size: 24px;
  font-weight: 800;
  color: #f8fafc;
}

.summary-desc-text {
  font-size: 14px;
  color: #94a3b8;
  max-width: 650px;
}

.summary-header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.project-tasks-preview {
  background: #111827;
  border: 1px solid rgba(59, 130, 246, 0.22);
  border-radius: 12px;
  padding: 20px;
}

.preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(59, 130, 246, 0.15);
}

.preview-header h3 {
  font-size: 16px;
  font-weight: 700;
  color: #f8fafc;
}

.tasks-table {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.task-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: #162033;
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
}

.task-row:hover {
  background: #1c2842;
  border-color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.2);
}

.task-row-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.task-row-title {
  font-size: 13px;
  font-weight: 600;
  color: #f8fafc;
}

.subtask-mini-pill {
  font-size: 11px;
  background: #1e293b;
  color: #38bdf8;
  padding: 2px 6px;
  border-radius: 4px;
  border: 1px solid rgba(56, 189, 248, 0.2);
}

.task-row-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-pill {
  font-size: 11px;
  font-weight: 700;
  background: #1e293b;
  color: #cbd5e1;
  padding: 2px 8px;
  border-radius: 4px;
}

/* ======================================================== */
/* VIEW 2: KANBAN BOARD STYLES                              */
/* ======================================================== */
.board-view-wrapper {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
  background: #0b0f19;
}

.board-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 24px;
  background: #111827;
  border-bottom: 1px solid rgba(59, 130, 246, 0.2);
}

.toolbar-left, .toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.board-container {
  display: flex;
  gap: 16px;
  padding: 20px 24px;
  flex: 1;
  overflow-x: auto;
  align-items: flex-start;
  background: #0b0f19;
}

.column {
  flex: 0 0 310px;
  background: #111827;
  border-radius: 10px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 80px);
  border: 1px solid rgba(59, 130, 246, 0.22);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.column-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 10px;
  margin-bottom: 8px;
  border-bottom: 2px solid rgba(59, 130, 246, 0.2);
}

.column-title-group { display: flex; align-items: center; gap: 8px; }

.column-title-group h3 {
  font-size: 14px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #cbd5e1;
}

.column-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
  box-shadow: 0 0 8px currentColor;
}

.dot-backlog { color: #94a3b8; background: #94a3b8; }
.dot-todo { color: #38bdf8; background: #38bdf8; }
.dot-in-progress { color: #f59e0b; background: #f59e0b; }
.dot-done { color: #10b981; background: #10b981; }
.dot-killed { color: #ef4444; background: #ef4444; }

.task-count {
  background: #1a2234;
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
  font-size: 12px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 12px;
}

.cards-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow-y: auto;
  padding-right: 2px;
  flex: 1;
}

/* Task Cards */
.card {
  background: #162033;
  border-radius: 8px;
  padding: 12px;
  border: 1px solid rgba(59, 130, 246, 0.2);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s, border-color 0.15s;
}

.card:hover {
  box-shadow: 0 0 12px rgba(56, 189, 248, 0.3);
  border-color: #38bdf8;
  transform: translateY(-2px);
}

.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.badge {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  padding: 2px 7px;
  border-radius: 4px;
}

.badge-urgent { background: rgba(239, 68, 68, 0.2); color: #fca5a5; border: 1px solid rgba(239, 68, 68, 0.4); }
.badge-high { background: rgba(249, 115, 22, 0.2); color: #fdba74; border: 1px solid rgba(249, 115, 22, 0.4); }
.badge-medium { background: rgba(59, 130, 246, 0.2); color: #93c5fd; border: 1px solid rgba(59, 130, 246, 0.4); }
.badge-low { background: rgba(148, 163, 184, 0.2); color: #cbd5e1; border: 1px solid rgba(148, 163, 184, 0.4); }

.btn-card-delete {
  background: transparent;
  border: none;
  font-size: 16px;
  color: #64748b;
  cursor: pointer;
}
.btn-card-delete:hover { color: #f87171; }

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: #f8fafc;
  margin-bottom: 4px;
}

.card-notes {
  font-size: 12px;
  color: #94a3b8;
  margin-bottom: 8px;
  white-space: pre-line;
}

/* Subtask Checklist Badge on Card */
.card-checklist-badge {
  margin: 6px 0;
  padding: 6px 8px;
  background: #111827;
  border-radius: 6px;
  border: 1px solid rgba(59, 130, 246, 0.15);
}

.checklist-info {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #38bdf8;
  font-weight: 600;
  margin-bottom: 4px;
}

.checklist-progress-bar {
  background: #1e293b;
  height: 4px;
  border-radius: 2px;
  overflow: hidden;
}

.checklist-progress-fill {
  background: #38bdf8;
  height: 100%;
  transition: width 0.3s ease;
}

.card-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 11px;
  color: #94a3b8;
  margin-bottom: 8px;
}

.due-tag {
  background: #1e293b;
  border: 1px solid rgba(59, 130, 246, 0.2);
  padding: 2px 6px;
  border-radius: 4px;
}

.card-move {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  font-size: 11px;
  border-top: 1px dashed rgba(59, 130, 246, 0.2);
  padding-top: 6px;
  margin-top: 4px;
  color: #94a3b8;
}

.card-move select {
  font-size: 11px;
  padding: 2px 4px;
  border-radius: 4px;
  border: 1px solid rgba(59, 130, 246, 0.25);
  background: #111827;
  color: #f1f5f9;
}

.empty-dropzone {
  border: 2px dashed rgba(59, 130, 246, 0.3);
  border-radius: 6px;
  padding: 20px;
  text-align: center;
  color: #64748b;
  font-size: 12px;
}

/* Modals */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: #111827;
  border: 1px solid rgba(56, 189, 248, 0.4);
  border-radius: 12px;
  width: 450px;
  max-width: 90vw;
  box-shadow: 0 0 25px rgba(56, 189, 248, 0.25), 0 20px 25px -5px rgba(0,0,0,0.6);
  overflow: hidden;
}

.modal-lg {
  width: 580px;
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(59, 130, 246, 0.2);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-header h2 { font-size: 16px; font-weight: 700; color: #f8fafc; }
.btn-close { background: none; border: none; font-size: 20px; cursor: pointer; color: #94a3b8; }
.btn-close:hover { color: #f87171; }

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 85vh;
  overflow-y: auto;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.form-row { display: flex; gap: 12px; }

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: #94a3b8;
}

.form-group input, .form-group select, .form-group textarea {
  padding: 8px 10px;
  border: 1px solid rgba(59, 130, 246, 0.25);
  background: #162033;
  color: #f1f5f9;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
}

.form-group input:focus, .form-group select:focus, .form-group textarea:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 8px rgba(56, 189, 248, 0.4);
}

/* Checklist / Subtasks UI in Modal */
.subtasks-section {
  background: #131b2a;
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 8px;
  padding: 14px;
  margin-top: 4px;
}

.subtasks-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.subtasks-header label {
  font-size: 13px;
  font-weight: 700;
  color: #38bdf8;
}

.subtask-progress-label {
  font-size: 11px;
  color: #94a3b8;
  font-weight: 600;
}

.subtask-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 10px 0;
  max-height: 180px;
  overflow-y: auto;
}

.subtask-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  background: #162033;
  border-radius: 6px;
  border: 1px solid rgba(59, 130, 246, 0.15);
  transition: all 0.15s;
}

.subtask-item:hover {
  border-color: rgba(56, 189, 248, 0.4);
}

.subtask-checkbox {
  width: 16px;
  height: 16px;
  accent-color: #38bdf8;
  cursor: pointer;
}

.subtask-title {
  flex: 1;
  font-size: 13px;
  color: #f1f5f9;
  cursor: pointer;
}

.subtask-done .subtask-title {
  text-decoration: line-through;
  color: #64748b;
}

.btn-subtask-del {
  background: transparent;
  border: none;
  color: #64748b;
  font-size: 16px;
  cursor: pointer;
}

.btn-subtask-del:hover {
  color: #f87171;
}

.add-subtask-box {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.add-subtask-box input {
  flex: 1;
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid rgba(59, 130, 246, 0.25);
  background: #162033;
  color: #f1f5f9;
  font-size: 12px;
}

/* Project Stat Cards in Modal */
.project-stat-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-top: 10px;
}

.stat-box {
  background: #162033;
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 8px;
  padding: 10px;
  display: flex;
  flex-direction: column;
}

.stat-number { font-size: 20px; font-weight: 800; }
.stat-title { font-size: 11px; font-weight: 600; color: #cbd5e1; margin-top: 2px; }
.stat-sub { font-size: 10px; color: #64748b; margin-top: 2px; }

.project-modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid rgba(59, 130, 246, 0.2);
}

.footer-left, .footer-right {
  display: flex;
  gap: 10px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 10px;
}

/* Toast */
.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  padding: 10px 18px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  z-index: 200;
  box-shadow: 0 0 16px rgba(0, 0, 0, 0.6);
}

.toast-success { background: #15803d; border: 1px solid #4ade80; box-shadow: 0 0 12px rgba(74, 222, 128, 0.3); }
.toast-error { background: #b91c1c; border: 1px solid #f87171; box-shadow: 0 0 12px rgba(248, 113, 113, 0.3); }
.toast-info { background: #1d4ed8; border: 1px solid #38bdf8; box-shadow: 0 0 12px rgba(56, 189, 248, 0.3); }
</style>
