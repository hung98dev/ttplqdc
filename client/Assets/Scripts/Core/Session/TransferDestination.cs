namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Destination carried into TRANSFERRING_MAP (attach_ok /
    /// transfer_prepare payloads per messages.md): the map group the
    /// client loads Addressables for, and its channel/instance target.
    /// </summary>
    public readonly struct TransferDestination
    {
        public TransferDestination(
            string mapId,
            uint channelIndex,
            byte[]? instanceId,
            string contentRevision)
        {
            MapId = mapId;
            ChannelIndex = channelIndex;
            InstanceId = instanceId;
            ContentRevision = contentRevision;
        }

        public string MapId
        {
            get;
        }

        public uint ChannelIndex
        {
            get;
        }

        public byte[]? InstanceId
        {
            get;
        }

        public string ContentRevision
        {
            get;
        }
    }
}
