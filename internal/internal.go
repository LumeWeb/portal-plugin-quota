package internal

const PluginName = "quota"
const ProtocolName = "quota"

// DEFAULT_REDUNDANCY is the baseline redundancy factor used for calculating
// normalized storage consumption. When actual redundancy differs from this value,
// the recorded storage bytes are scaled proportionally.
// A value of 1.0 means no redundancy (1 copy), higher values mean more redundancy.
const DEFAULT_REDUNDANCY = 3.0
