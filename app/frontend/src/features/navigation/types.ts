export interface NavigationChildItem {
  label: string
  to: string
}

export interface NavigationItem extends NavigationChildItem {
  children?: NavigationChildItem[]
}
