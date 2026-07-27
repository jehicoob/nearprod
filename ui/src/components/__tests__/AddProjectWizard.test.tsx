import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { AddProjectWizard } from '../AddProjectWizard'

describe('AddProjectWizard', () => {
  it('shows source options and safety notice', () => {
    render(<AddProjectWizard open onClose={vi.fn()} onDetect={vi.fn()} onSubmit={vi.fn()} />)
    expect(screen.getByText('New Project Wizard')).toBeInTheDocument()
    expect(screen.getByText('Local Folder')).toBeInTheDocument()
    expect(screen.getByText(/Repositorios de terceros/)).toBeInTheDocument()
  })
})
