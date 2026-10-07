Shader "ThinhThan/Perf/OverdrawCount"
{
    // Test-only overdraw counter (client_performance.md PERF-016): additive
    // One One blend writes exactly +1 in the red channel for every fragment
    // of the rendered mesh area — including transparent margins — so a
    // single-channel float target accumulates the overdraw count per pixel.
    SubShader
    {
        Tags { "RenderType" = "Transparent" "Queue" = "Transparent" }
        Blend One One
        Cull Off
        ZWrite Off
        ZTest Always

        Pass
        {
            HLSLPROGRAM
            #pragma vertex Vert
            #pragma fragment Frag
            #include "Packages/com.unity.render-pipelines.universal/ShaderLibrary/Core.hlsl"

            struct Attributes
            {
                float4 positionOS : POSITION;
            };

            struct Varyings
            {
                float4 positionCS : SV_POSITION;
            };

            Varyings Vert(Attributes input)
            {
                Varyings output;
                output.positionCS = TransformObjectToHClip(input.positionOS.xyz);
                return output;
            }

            half4 Frag(Varyings input) : SV_Target
            {
                return half4(1, 0, 0, 1);
            }

            ENDHLSL
        }
    }
}
