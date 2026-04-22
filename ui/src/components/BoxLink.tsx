export const BaseBoxLink = forwardRef<HTMLAnchorElement, BoxProps>((props, ref) => {
    // Use 'as="a"' or 'component="a"' depending on your library's API
    return <Box ref={ref} as="a" {...props} />
})