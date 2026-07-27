import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LogsModal } from '../LogsModal'

const baseProps = {
  open: true,
  title: 'Backend API · backend',
  projectId: 'pos_backend',
  output: 'INFO server started\nERROR database failed\nINFO retrying database',
  isLoading: false,
  isRefreshing: false,
  error: null,
  lastUpdated: '2026-06-09T04:00:00.000Z',
  onClose: vi.fn(),
  onRefresh: vi.fn(),
  onLiveRefresh: vi.fn(),
}

describe('LogsModal', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('searches, navigates and filters logs internally', async () => {
    const user = userEvent.setup()
    render(<LogsModal {...baseProps} />)

    expect(screen.getByRole('dialog', { name: /logs de backend api/i })).toBeInTheDocument()
    await user.type(screen.getByPlaceholderText('Buscar dentro de los logs...'), 'database')
    expect(screen.getByText('1/2 coincidencias')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /siguiente coincidencia/i }))
    expect(screen.getByText('2/2 coincidencias')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /coincidencia anterior/i }))
    expect(screen.getByText('1/2 coincidencias')).toBeInTheDocument()

    await user.click(screen.getByLabelText(/solo coincidencias/i))
    expect(screen.queryByText(/server started/)).not.toBeInTheDocument()
    expect(screen.getByText(/ERROR/)).toBeInTheDocument()
  })

  it('polls logs when live mode is enabled', () => {
    vi.useFakeTimers()
    const onLiveRefresh = vi.fn()
    render(<LogsModal {...baseProps} onLiveRefresh={onLiveRefresh} />)

    fireEvent.click(screen.getByLabelText(/live/i))
    act(() => {
      vi.advanceTimersByTime(2100)
    })

    expect(onLiveRefresh).toHaveBeenCalledTimes(1)
  })
})
