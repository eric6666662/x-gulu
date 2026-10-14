import { fireEvent, render, screen } from '@testing-library/vue'
import { describe, expect, it } from 'vitest'

import type { Conversation } from '@/types'

import ConversationList from './ConversationList.vue'

const items: Conversation[] = [
  {
    id: 'c2',
    title: '今天聊点什么',
    serviceId: null,
    modelId: null,
    roleId: null,
    messageCount: 4,
    createdAt: 1_760_000_000,
    updatedAt: 1_760_000_200,
  },
  {
    id: 'c1',
    title: '写周报',
    serviceId: null,
    modelId: null,
    roleId: null,
    messageCount: 1,
    createdAt: 1_760_000_000,
    updatedAt: 1_760_000_100,
  },
]

interface ListProps {
  items: Conversation[]
  activeId: string | null
  loading: boolean
  error: string | null
}

function renderList(props: ListProps) {
  return render(ConversationList, { props })
}

describe('ConversationList', () => {
  it('没有会话时给出引导用户点「新会话」的空态', () => {
    renderList({ items: [], activeId: null, loading: false, error: null })

    expect(screen.getByText('还没有会话，点上方「新会话」开始')).toBeTruthy()
  })

  it('加载中显示加载态而不是空态', () => {
    renderList({ items: [], activeId: null, loading: true, error: null })

    expect(screen.getByText('正在加载会话…')).toBeTruthy()
    expect(screen.queryByText('还没有会话，点上方「新会话」开始')).toBeNull()
  })

  it('出错时显示错误文案与重试入口', () => {
    renderList({ items: [], activeId: null, loading: false, error: '连不上服务，请确认后端已启动' })

    expect(screen.getByText('连不上服务，请确认后端已启动')).toBeTruthy()
    expect(screen.getByRole('button', { name: '重试' })).toBeTruthy()
  })

  it('有会话时按传入顺序渲染标题', () => {
    renderList({ items, activeId: null, loading: false, error: null })

    const links = screen.getAllByRole('link')
    expect(links.map((el) => el.textContent?.trim())).toEqual(['今天聊点什么', '写周报'])
  })

  it('把当前会话标成 aria-current', () => {
    renderList({ items, activeId: 'c1', loading: false, error: null })

    const current = screen.getByRole('link', { name: '写周报' })
    expect(current.getAttribute('aria-current')).toBe('page')
    expect(screen.getByRole('link', { name: '今天聊点什么' }).getAttribute('aria-current')).toBeNull()
  })

  it('点某条会话时把 id 抛给父组件', async () => {
    const { emitted } = renderList({ items, activeId: null, loading: false, error: null })

    await fireEvent.click(screen.getByRole('link', { name: '今天聊点什么' }))

    expect(emitted('select')?.[0]).toEqual(['c2'])
  })

  it('点重试时抛 retry', async () => {
    const { emitted } = renderList({ items: [], activeId: null, loading: false, error: '连接失败' })

    await fireEvent.click(screen.getByRole('button', { name: '重试' }))

    expect(emitted('retry')).toBeTruthy()
  })
})
