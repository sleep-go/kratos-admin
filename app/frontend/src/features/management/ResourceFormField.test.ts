import { mount } from '@vue/test-utils'

import ResourceFormField from './ResourceFormField.vue'

const options = [
  { value: '1', label: '研发部 · ID 1', children: [{ value: '2', label: '研发一组 · ID 2' }] }
]

function mountField(type: 'select' | 'relation' | 'tree') {
  return mount(ResourceFormField, {
    props: {
      modelValue: type === 'select' ? 'email' : 1,
      field: {
        key: 'target',
        label: '目标',
        type,
        options: type === 'select' ? [{ value: 'email', label: '邮件' }] : undefined
      },
      options
    },
    global: {
      stubs: {
        ElSelect: {
          name: 'ElSelect',
          props: ['modelValue'],
          template: '<select data-testid="select"><slot /></select>'
        },
        ElOption: {
          props: ['label', 'value'],
          template: '<option :value="value">{{ label }}</option>'
        },
        ElTreeSelect: {
          name: 'ElTreeSelect',
          props: ['modelValue', 'data'],
          template: '<div data-testid="tree-select" :data-count="data.length" />'
        }
      }
    }
  })
}

describe('通用资源表单字段', () => {
  it('枚举和关联字段渲染为下拉选择', () => {
    expect(mountField('select').get('[data-testid="select"]').text()).toContain('邮件')
    expect(mountField('relation').get('[data-testid="select"]').text()).toContain('研发部')
  })

  it('层级关联字段渲染为树选择', () => {
    const wrapper = mountField('tree')

    expect(wrapper.get('[data-testid="tree-select"]').attributes('data-count')).toBe('1')
    expect(wrapper.getComponent({ name: 'ElTreeSelect' }).props('modelValue')).toBe('1')
  })

  it('按选项值语义匹配字符串和数字，避免编辑时只回显原始数字', () => {
    const menuType = mount(ResourceFormField, {
      props: {
        modelValue: '2',
        field: {
          key: 'type',
          label: '类型',
          type: 'select',
          options: [
            { value: 1, label: '目录' },
            { value: 2, label: '菜单' }
          ]
        }
      },
      global: {
        stubs: {
          ElSelect: {
            name: 'ElSelect',
            props: ['modelValue'],
            template: '<select><slot /></select>'
          },
          ElOption: true
        }
      }
    })
    const relation = mount(ResourceFormField, {
      props: {
        modelValue: 1,
        field: { key: 'user_id', label: '用户', type: 'relation' },
        options: [{ value: '1', label: '管理员' }]
      },
      global: {
        stubs: {
          ElSelect: {
            name: 'ElSelect',
            props: ['modelValue'],
            template: '<select><slot /></select>'
          },
          ElOption: true
        }
      }
    })

    expect(menuType.getComponent({ name: 'ElSelect' }).props('modelValue')).toBe(2)
    expect(relation.getComponent({ name: 'ElSelect' }).props('modelValue')).toBe('1')
  })
})
