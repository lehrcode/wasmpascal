program MitInclude;
{$DEFINE AUSFUEHRLICH}
const
{$I 33_include.inc}
var
  i: Integer;
begin
{$IFDEF AUSFUEHRLICH}
  WriteLn('Version ', Version);
{$ELSE}
  WriteLn('still');
{$ENDIF}
{$IFNDEF VER50}
  das hier wird nie gelesen ä ö ü
{$ENDIF}
  for i := 1 to Anzahl do Write(i)
end.
