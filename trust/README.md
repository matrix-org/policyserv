# Developer notes: Trust Sources

Trust Sources (`trust.Source` implementations) are expected to be able to be created randomly and with minimal context,
unlike filters which require a known community state. This is primarily so that API endpoints can be used to populate the
trust data in the source: instead of having the API layer track down an already-created source, it can just create a new
one, run the new data through it, and then discard the object for the garbage collector.

It's incredibly important that trust sources **do not** assume that they only have a single instance running, or that
their lifespan is long. Sources which need to persist data across objects **must** have a management layer they plug
into rather than create.

Trust sources can store data in the database using `storage.PersistentStorage.[Get/Set]TrustData`. This data is keyed by
the source's "name" (arbitrary string - pick something unique for the source) and a "key". The key is arbitrary and 
intended to be used for stuff like a room/community ID. The key may be an empty string if the source doesn't scope its
data. Note that the data is stored as JSON in the database.
