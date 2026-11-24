program DisposeNil;
var
  p: ^Integer;
begin
  p := nil;
  Dispose(p)
end.
