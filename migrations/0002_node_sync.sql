ALTER TABLE nodes
  ADD CONSTRAINT nodes_headscale_node_id_unique UNIQUE (headscale_node_id);

